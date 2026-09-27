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
	DeploymentID string
	ServiceID    string
	ServiceSlug  string
	RepoURL      string
	Branch       string
	CommitSHA    string
	BuildMethod  string // "nixpacks" or "dockerfile"
	EnvVars      map[string]string
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
		branch := opts.Branch
		if branch == "" {
			branch = "main"
		}

		cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, opts.RepoURL, workDir)
		cloneCmd.Stdout = logWriter
		cloneCmd.Stderr = logWriter
		if err := cloneCmd.Run(); err != nil {
			appendLog(fmt.Sprintf("[klouds-builder] Git clone failed: %v", err))
			return &BuildResult{
				ImageTag: imageTag,
				Duration: time.Since(startTime),
				Logs:     logBuf.String(),
				Success:  false,
				Error:    err,
			}, err
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

	// Step 3: Execute build based on method
	var buildErr error
	dockerfilePath := filepath.Join(workDir, "Dockerfile")
	hasDockerfile := fileExists(dockerfilePath)

	if opts.BuildMethod == "dockerfile" || (opts.BuildMethod == "auto" && hasDockerfile) {
		appendLog("[klouds-builder] Building via Dockerfile...")
		buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageTag, workDir)
		buildCmd.Stdout = logWriter
		buildCmd.Stderr = logWriter
		buildErr = buildCmd.Run()
	} else {
		// Use Nixpacks
		appendLog("[klouds-builder] Building via Nixpacks engine...")
		if isCommandAvailable("nixpacks") {
			nixArgs := []string{"build", workDir, "--name", imageTag}
			for k, v := range opts.EnvVars {
				nixArgs = append(nixArgs, "--env", fmt.Sprintf("%s=%s", k, v))
			}
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
				"-v", fmt.Sprintf("%s:/app:ro", workDir),
				"ghcr.io/railwayapp/nixpacks:latest",
				"build", "/app", "--name", imageTag,
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
