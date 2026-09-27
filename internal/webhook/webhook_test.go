package webhook

import (
	"testing"
)

func TestIsPathAffected(t *testing.T) {
	tests := []struct {
		name         string
		rootDir      string
		changedFiles map[string]bool
		expected     bool
	}{
		{
			name:         "Root service affected by any change",
			rootDir:      ".",
			changedFiles: map[string]bool{"backend/main.go": true},
			expected:     true,
		},
		{
			name:         "Subdirectory matches changed file",
			rootDir:      "backend",
			changedFiles: map[string]bool{"backend/server.go": true, "README.md": true},
			expected:     true,
		},
		{
			name:         "Subdirectory with dot-slash matches changed file",
			rootDir:      "./backend",
			changedFiles: map[string]bool{"backend/handlers/api.go": true},
			expected:     true,
		},
		{
			name:         "Subdirectory not touched is not affected",
			rootDir:      "frontend",
			changedFiles: map[string]bool{"backend/main.go": true, "db/init.sql": true},
			expected:     false,
		},
		{
			name:         "Partial name prefix does not match",
			rootDir:      "app",
			changedFiles: map[string]bool{"application/main.go": true},
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathAffected(tt.rootDir, tt.changedFiles)
			if got != tt.expected {
				t.Errorf("isPathAffected(%q, %v) = %v; want %v", tt.rootDir, tt.changedFiles, got, tt.expected)
			}
		})
	}
}

func TestMatchesRepoURL(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		cloneURL   string
		htmlURL    string
		expected   bool
	}{
		{
			name:       "HTTPS clone URL match",
			configured: "https://github.com/myorg/myrepo",
			cloneURL:   "https://github.com/myorg/myrepo.git",
			htmlURL:    "https://github.com/myorg/myrepo",
			expected:   true,
		},
		{
			name:       "SSH clone URL match",
			configured: "git@github.com:myorg/myrepo.git",
			cloneURL:   "https://github.com/myorg/myrepo.git",
			htmlURL:    "https://github.com/myorg/myrepo",
			expected:   true,
		},
		{
			name:       "Different repo does not match",
			configured: "https://github.com/myorg/other-repo",
			cloneURL:   "https://github.com/myorg/myrepo.git",
			htmlURL:    "https://github.com/myorg/myrepo",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesRepoURL(tt.configured, tt.cloneURL, tt.htmlURL)
			if got != tt.expected {
				t.Errorf("matchesRepoURL(%q, %q, %q) = %v; want %v", tt.configured, tt.cloneURL, tt.htmlURL, got, tt.expected)
			}
		})
	}
}
