package blueprint

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DetectionResult contains the detected blueprint and discovery source.
type DetectionResult struct {
	Blueprint *Blueprint `json:"blueprint"`
	Source    string     `json:"source"` // "blueprint", "docker_compose", "monorepo_detected", "single_detected"
}

// DetectFromRepo clones a repository shallowly to inspect its structure and discover services.
func DetectFromRepo(ctx context.Context, repoURL, branch, dataDir string) (*DetectionResult, error) {
	if branch == "" {
		branch = "main"
	}

	targetBase := dataDir
	if targetBase != "" {
		if err := os.MkdirAll(targetBase, 0755); err != nil {
			targetBase = os.TempDir()
		}
	} else {
		targetBase = os.TempDir()
	}

	tempDir, err := os.MkdirTemp(targetBase, "klouds-detect-*")
	if err != nil {
		tempDir, err = os.MkdirTemp("", "klouds-detect-*")
		if err != nil {
			return nil, fmt.Errorf("create temp detection dir: %w", err)
		}
	}
	defer os.RemoveAll(tempDir)

	// Shallow clone
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", branch, repoURL, tempDir)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		// Try without branch if branch failed
		cloneCmdFallback := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, tempDir)
		if fallbackOut, fbErr := cloneCmdFallback.CombinedOutput(); fbErr != nil {
			return nil, fmt.Errorf("git clone failed: %s (fallback: %s)", string(out), string(fallbackOut))
		}
	}

	return DetectFromDirectory(tempDir)
}

// DetectFromDirectory inspects an extracted repository directory for blueprints or service patterns.
func DetectFromDirectory(rootPath string) (*DetectionResult, error) {
	// 1. Check for explicit blueprint files
	blueprintCandidates := []string{
		"klouds.yaml", "klouds.yml",
		"render.yaml", "render.yml",
		"devpanel.yaml", "devpanel.yml",
		"paas.yaml", "paas.yml",
	}

	for _, file := range blueprintCandidates {
		p := filepath.Join(rootPath, file)
		if data, err := os.ReadFile(p); err == nil {
			bp, err := ParseBlueprint(data)
			if err == nil && len(bp.Services) > 0 {
				return &DetectionResult{
					Blueprint: bp,
					Source:    "blueprint",
				}, nil
			}
		}
	}

	// 2. Check for docker-compose.yml
	composeCandidates := []string{
		"docker-compose.yml", "docker-compose.yaml",
		"compose.yml", "compose.yaml",
	}

	for _, file := range composeCandidates {
		p := filepath.Join(rootPath, file)
		if data, err := os.ReadFile(p); err == nil {
			if bp, err := parseComposeFile(data, rootPath); err == nil && len(bp.Services) > 0 {
				return &DetectionResult{
					Blueprint: bp,
					Source:    "docker_compose",
				}, nil
			}
		}
	}

	// 3. Monorepo and multi-directory scanner
	detectedServices := scanDirectoriesForServices(rootPath)
	if len(detectedServices) > 0 {
		source := "single_detected"
		if len(detectedServices) > 1 {
			source = "monorepo_detected"
		}

		bp := &Blueprint{
			Version:  "1",
			Services: detectedServices,
		}
		return &DetectionResult{
			Blueprint: bp,
			Source:    source,
		}, nil
	}

	return nil, fmt.Errorf("no supported services or blueprints detected in repository")
}

func parseComposeFile(data []byte, rootPath string) (*Blueprint, error) {
	var compose struct {
		Services map[string]struct {
			Build interface{} `yaml:"build"`
			Image string      `yaml:"image"`
			Ports []string    `yaml:"ports"`
		} `yaml:"services"`
	}

	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, err
	}

	bp := &Blueprint{Version: "1"}
	for name, svc := range compose.Services {
		def := ServiceDefinition{
			Name:        name,
			Type:        "web",
			Env:         "docker",
			BuildMethod: "dockerfile",
			Port:        3000,
		}

		// Check build context
		switch b := svc.Build.(type) {
		case string:
			def.RootDir = b
		case map[string]interface{}:
			if ctx, ok := b["context"].(string); ok {
				def.RootDir = ctx
			}
			if df, ok := b["dockerfile"].(string); ok {
				def.DockerfilePath = df
			}
		}

		if def.RootDir == "" {
			def.RootDir = "."
		}

		bp.Services = append(bp.Services, def)
	}

	return bp, nil
}

func scanDirectoriesForServices(rootPath string) []ServiceDefinition {
	var services []ServiceDefinition

	// Potential monorepo subdirectories to check
	candidateDirs := []string{"."}

	entries, err := os.ReadDir(rootPath)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}

			// Subdirectories like apps, services, packages
			if e.Name() == "apps" || e.Name() == "services" || e.Name() == "packages" {
				nested, nErr := os.ReadDir(filepath.Join(rootPath, e.Name()))
				if nErr == nil {
					for _, ne := range nested {
						if ne.IsDir() && !strings.HasPrefix(ne.Name(), ".") {
							candidateDirs = append(candidateDirs, filepath.Join(e.Name(), ne.Name()))
						}
					}
				}
			} else {
				// Direct subdirectories like backend, frontend, api, web
				candidateDirs = append(candidateDirs, e.Name())
			}
		}
	}

	for _, relDir := range candidateDirs {
		fullPath := filepath.Join(rootPath, relDir)
		svc, ok := inspectServiceDirectory(rootPath, relDir, fullPath)
		if ok {
			services = append(services, svc)
		}
	}

	// If subdirectories were found, filter out root if it was just a monorepo container
	if len(services) > 1 {
		filtered := make([]ServiceDefinition, 0, len(services))
		for _, s := range services {
			if s.RootDir != "." {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) > 0 {
			return filtered
		}
	}

	return services
}

func inspectServiceDirectory(rootPath, relDir, fullPath string) (ServiceDefinition, bool) {
	name := filepath.Base(fullPath)
	if relDir == "." {
		name = filepath.Base(rootPath)
	}
	name = sanitizeServiceName(name)

	// Check for Dockerfile
	if fileExists(filepath.Join(fullPath, "Dockerfile")) {
		return ServiceDefinition{
			Name:        name,
			Type:        "web",
			Env:         "docker",
			BuildMethod: "dockerfile",
			RootDir:     relDir,
			Port:        3000,
		}, true
	}

	// Check for Node.js
	if fileExists(filepath.Join(fullPath, "package.json")) {
		port := int32(3000)
		if strings.Contains(name, "api") || strings.Contains(name, "backend") {
			port = 8080
		}
		return ServiceDefinition{
			Name:         name,
			Type:         "web",
			Env:          "node",
			BuildMethod:  "nixpacks",
			RootDir:      relDir,
			BuildCommand: "npm run build",
			StartCommand: "npm start",
			Port:         port,
		}, true
	}

	// Check for Go
	if fileExists(filepath.Join(fullPath, "go.mod")) {
		return ServiceDefinition{
			Name:         name,
			Type:         "web",
			Env:          "go",
			BuildMethod:  "nixpacks",
			RootDir:      relDir,
			BuildCommand: "go build -o server .",
			StartCommand: "./server",
			Port:         8080,
		}, true
	}

	// Check for Python
	if fileExists(filepath.Join(fullPath, "requirements.txt")) ||
		fileExists(filepath.Join(fullPath, "pyproject.toml")) ||
		fileExists(filepath.Join(fullPath, "Pipfile")) {
		return ServiceDefinition{
			Name:        name,
			Type:        "web",
			Env:         "python",
			BuildMethod: "nixpacks",
			RootDir:     relDir,
			Port:        8000,
		}, true
	}

	// Check for Rust
	if fileExists(filepath.Join(fullPath, "Cargo.toml")) {
		return ServiceDefinition{
			Name:        name,
			Type:        "web",
			Env:         "rust",
			BuildMethod: "nixpacks",
			RootDir:     relDir,
			Port:        8080,
		}, true
	}

	return ServiceDefinition{}, false
}

func sanitizeServiceName(name string) string {
	name = strings.ToLower(name)
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			sb.WriteRune(r)
		} else if r == '_' || r == ' ' || r == '/' || r == '\\' {
			sb.WriteRune('-')
		}
	}
	res := strings.Trim(sb.String(), "-")
	if res == "" {
		res = "app"
	}
	return res
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
