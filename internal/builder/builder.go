// Package builder coordinates Nixpacks and Dockerfile builds for services.
package builder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vedant/klouds/internal/db"
)

// Engine orchestrates building container images from source.
type Engine struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	dataDir string
}

// NewEngine creates a new build engine instance.
func NewEngine(pool *pgxpool.Pool, dataDir string) *Engine {
	return &Engine{
		pool:    pool,
		queries: db.New(pool),
		dataDir: dataDir,
	}
}

// BuildOptions contains inputs for a service build execution.
type BuildOptions struct {
	DeploymentID   string
	ServiceID      string
	ServiceSlug    string
	RepoURL        string
	Branch         string
	CommitSHA      string
	BuildMethod    string // "nixpacks" or "dockerfile"
	RootDir        string // Subdirectory in repository for monorepo support
	DockerfilePath string
	BuildCommand   string
	StartCommand   string
	RuntimeVersion string
	EnvVars        map[string]string
}

// BuildResult holds the output of a completed build.
type BuildResult struct {
	ImageTag string
	Duration time.Duration
	Logs     string
	Success  bool
	Error    error
}

// Build executes the build process, capturing streaming logs.
func (e *Engine) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	startTime := time.Now()
	shortID := opts.DeploymentID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	imageTag := fmt.Sprintf("klouds/%s:%s", opts.ServiceSlug, shortID)

	workDir := filepath.Join(e.dataDir, "builds", opts.DeploymentID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create build directory: %w", err)
	}
	defer func() {
		// Clean up build directory after completion
		_ = os.RemoveAll(workDir)
	}()

	var logBuf bytes.Buffer
	logWriter := &synchronizedWriter{writer: &logBuf}

	appendLog := func(line string) {
		logWriter.Write([]byte(line + "\n"))
		_ = e.queries.UpdateDeploymentLog(ctx, opts.DeploymentID, logBuf.String())
	}

	appendLog(fmt.Sprintf("[klouds-builder] Starting build for deployment %s", opts.DeploymentID))
	appendLog(fmt.Sprintf("[klouds-builder] Target image tag: %s", imageTag))

	// Step 1: Clone Repository
	if opts.RepoURL != "" {
		appendLog(fmt.Sprintf("[klouds-builder] Cloning %s (branch: %s)...", opts.RepoURL, opts.Branch))
		branch := strings.TrimSpace(opts.Branch)
		var cloneErr error

		if branch != "" {
			cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, opts.RepoURL, workDir)
			cloneCmd.Stdout = logWriter
			cloneCmd.Stderr = logWriter
			cloneErr = cloneCmd.Run()
		} else {
			cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", opts.RepoURL, workDir)
			cloneCmd.Stdout = logWriter
			cloneCmd.Stderr = logWriter
			cloneErr = cloneCmd.Run()
		}

		if cloneErr != nil {
			appendLog(fmt.Sprintf("[klouds-builder] Warning: Clone with branch %q failed: %v. Attempting clone of default branch...", branch, cloneErr))
			_ = os.RemoveAll(workDir)
			_ = os.MkdirAll(workDir, 0755)

			fallbackCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", opts.RepoURL, workDir)
			fallbackCmd.Stdout = logWriter
			fallbackCmd.Stderr = logWriter
			if fbErr := fallbackCmd.Run(); fbErr != nil {
				appendLog(fmt.Sprintf("[klouds-builder] Git clone failed: %v", fbErr))
				return &BuildResult{
					ImageTag: imageTag,
					Duration: time.Since(startTime),
					Logs:     logBuf.String(),
					Success:  false,
					Error:    fbErr,
				}, fbErr
			}
		}
		appendLog("[klouds-builder] Git clone successful.")
	}

	// Step 2: Checkout specific commit if requested
	if opts.CommitSHA != "" {
		appendLog(fmt.Sprintf("[klouds-builder] Checking out commit %s...", opts.CommitSHA))
		checkoutCmd := exec.CommandContext(ctx, "git", "-C", workDir, "checkout", opts.CommitSHA)
		checkoutCmd.Stdout = logWriter
		checkoutCmd.Stderr = logWriter
		_ = checkoutCmd.Run()
	}

	// Step 3: Determine build directory (monorepo root directory support)
	buildDir := workDir
	if opts.RootDir != "" && opts.RootDir != "." {
		buildDir = filepath.Join(workDir, opts.RootDir)
		appendLog(fmt.Sprintf("[klouds-builder] Monorepo subdirectory configured: %s", opts.RootDir))
	}

	// Determine Dockerfile path
	dfName := "Dockerfile"
	if opts.DockerfilePath != "" {
		dfName = opts.DockerfilePath
	}
	actualDockerfilePath := filepath.Join(buildDir, dfName)
	if !fileExists(actualDockerfilePath) {
		actualDockerfilePath = filepath.Join(workDir, dfName)
	}
	hasDockerfile := fileExists(actualDockerfilePath)

	// Step 4: Execute build based on method
	var buildErr error
	if opts.BuildMethod == "dockerfile" || (opts.BuildMethod == "auto" && hasDockerfile) {
		appendLog(fmt.Sprintf("[klouds-builder] Building via Dockerfile (%s)...", actualDockerfilePath))
		buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageTag, "-f", actualDockerfilePath, buildDir)
		buildCmd.Stdout = logWriter
		buildCmd.Stderr = logWriter
		buildErr = buildCmd.Run()
	} else {
		// Use Nixpacks
		appendLog("[klouds-builder] Building via Nixpacks engine...")
		nixArgs := []string{"build", buildDir, "--name", imageTag}
		buildCmd := opts.BuildCommand
		if buildCmd != "" {
			// Strip redundant npm install from buildCmd since Nixpacks already executes the install phase
			if strings.HasPrefix(buildCmd, "npm install && ") {
				buildCmd = strings.TrimPrefix(buildCmd, "npm install && ")
			} else if strings.HasPrefix(buildCmd, "npm i && ") {
				buildCmd = strings.TrimPrefix(buildCmd, "npm i && ")
			} else if strings.HasPrefix(buildCmd, "npm ci && ") {
				buildCmd = strings.TrimPrefix(buildCmd, "npm ci && ")
			}
			nixArgs = append(nixArgs, "--build-cmd", buildCmd)
		}
		if opts.StartCommand != "" {
			nixArgs = append(nixArgs, "--start-cmd", opts.StartCommand)
		}

		effectiveEnvVars := make(map[string]string)
		for k, v := range opts.EnvVars {
			effectiveEnvVars[k] = v
		}

		// Inject selected runtime version into Nixpacks configuration
		if opts.RuntimeVersion != "" {
			v := strings.TrimSpace(opts.RuntimeVersion)
			if fileExists(filepath.Join(buildDir, "package.json")) {
				effectiveEnvVars["NIXPACKS_NODE_VERSION"] = v
			} else if fileExists(filepath.Join(buildDir, "requirements.txt")) || fileExists(filepath.Join(buildDir, "Pipfile")) || fileExists(filepath.Join(buildDir, "pyproject.toml")) {
				effectiveEnvVars["NIXPACKS_PYTHON_VERSION"] = v
			} else if fileExists(filepath.Join(buildDir, "go.mod")) {
				effectiveEnvVars["NIXPACKS_GO_VERSION"] = v
			} else if fileExists(filepath.Join(buildDir, "composer.json")) {
				effectiveEnvVars["NIXPACKS_PHP_VERSION"] = v
			} else if fileExists(filepath.Join(buildDir, "Gemfile")) {
				effectiveEnvVars["NIXPACKS_RUBY_VERSION"] = v
			} else if fileExists(filepath.Join(buildDir, "pom.xml")) || fileExists(filepath.Join(buildDir, "build.gradle")) {
				effectiveEnvVars["NIXPACKS_JDK_VERSION"] = v
			} else if fileExists(filepath.Join(buildDir, "Cargo.toml")) {
				effectiveEnvVars["NIXPACKS_RUST_VERSION"] = v
			} else {
				effectiveEnvVars["NIXPACKS_NODE_VERSION"] = v
			}
		}

		var installCmd string
		if fileExists(filepath.Join(buildDir, "package.json")) {
			if _, ok := effectiveEnvVars["NIXPACKS_NODE_VERSION"]; !ok {
				if _, ok2 := effectiveEnvVars["NODE_VERSION"]; !ok2 {
					effectiveEnvVars["NIXPACKS_NODE_VERSION"] = "22"
				}
			}
			// Ensure npm uses --include=optional and installs native arm64 binding for rolldown/vite
			if !fileExists(filepath.Join(buildDir, "pnpm-lock.yaml")) && !fileExists(filepath.Join(buildDir, "yarn.lock")) {
				installCmd = "npm install --include=optional && (npm install @rolldown/binding-linux-arm64-gnu || true)"
			}
		}

		if installCmd != "" {
			nixArgs = append(nixArgs, "--install-cmd", installCmd)
		}

		for k, v := range effectiveEnvVars {
			nixArgs = append(nixArgs, "--env", fmt.Sprintf("%s=%s", k, v))
		}

		if isCommandAvailable("nixpacks") {
			nixCmd := exec.CommandContext(ctx, "nixpacks", nixArgs...)
			nixCmd.Stdout = logWriter
			nixCmd.Stderr = logWriter
			buildErr = nixCmd.Run()
		} else {
			// Fallback: Run nixpacks inside Docker container
			appendLog("[klouds-builder] Host nixpacks binary not detected. Executing containerized nixpacks...")
			dockerArgs := []string{
				"run", "--rm",
				"-v", "/var/run/docker.sock:/var/run/docker.sock",
				"-v", fmt.Sprintf("%s:/app:ro", buildDir),
				"ghcr.io/railwayapp/nixpacks:latest",
				"build", "/app", "--name", imageTag,
			}
			if installCmd != "" {
				dockerArgs = append(dockerArgs, "--install-cmd", installCmd)
			}
			if buildCmd != "" {
				dockerArgs = append(dockerArgs, "--build-cmd", buildCmd)
			}
			if opts.StartCommand != "" {
				dockerArgs = append(dockerArgs, "--start-cmd", opts.StartCommand)
			}
			for k, v := range effectiveEnvVars {
				dockerArgs = append(dockerArgs, "--env", fmt.Sprintf("%s=%s", k, v))
			}
			nixDockerCmd := exec.CommandContext(ctx, "docker", dockerArgs...)
			nixDockerCmd.Stdout = logWriter
			nixDockerCmd.Stderr = logWriter
			buildErr = nixDockerCmd.Run()
		}
	}

	duration := time.Since(startTime)
	if buildErr != nil {
		appendLog(fmt.Sprintf("[klouds-builder] Build failed: %v", buildErr))
		return &BuildResult{
			ImageTag: imageTag,
			Duration: duration,
			Logs:     logBuf.String(),
			Success:  false,
			Error:    buildErr,
		}, buildErr
	}

	appendLog(fmt.Sprintf("[klouds-builder] Build finished successfully in %v.", duration.Round(time.Millisecond)))
	return &BuildResult{
		ImageTag: imageTag,
		Duration: duration,
		Logs:     logBuf.String(),
		Success:  true,
		Error:    nil,
	}, nil
}

type synchronizedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *synchronizedWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
