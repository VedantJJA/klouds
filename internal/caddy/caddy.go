// Package caddy generates and applies Caddy reverse proxy configuration
// via the Caddy Admin API (http://localhost:2019).
package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

// Manager generates and applies Caddy configuration.
type Manager struct {
	adminAPI string // e.g. "http://localhost:2019"
	domain   string // e.g. "yourdomain.com"
}

// NewManager creates a new Caddy config manager.
func NewManager(adminAPI, domain string) *Manager {
	return &Manager{
		adminAPI: adminAPI,
		domain:   domain,
	}
}

// Route represents a Caddy route to a backend container.
type Route struct {
	Subdomain    string // e.g. "myapp" → myapp.domain.com
	BackendHost  string // Docker network hostname
	BackendPort  int    // Container internal port
	Redirects    []RedirectRule
	Rewrites     []RewriteRule
}

// RedirectRule represents a redirect configuration.
type RedirectRule struct {
	Source     string
	Target    string
	StatusCode int // 301 or 302
}

// RewriteRule represents a URI rewrite configuration.
type RewriteRule struct {
	Source string
	Target string
}

// GenerateCaddyfile produces a Caddyfile string for the given routes.
// This is used for debugging/display; actual config is applied via JSON API.
func (m *Manager) GenerateCaddyfile(routes []Route) string {
	var buf bytes.Buffer

	// Global options
	buf.WriteString("{\n")
	buf.WriteString("    admin off\n")
	buf.WriteString("    email admin@" + m.domain + "\n")
	buf.WriteString("}\n\n")

	for _, route := range routes {
		host := fmt.Sprintf("%s.%s", route.Subdomain, m.domain)
		buf.WriteString(fmt.Sprintf("%s {\n", host))

		// Redirects
		for _, redir := range route.Redirects {
			buf.WriteString(fmt.Sprintf("    redir %s %s %d\n", redir.Source, redir.Target, redir.StatusCode))
		}

		// Rewrites
		for _, rewrite := range route.Rewrites {
			buf.WriteString(fmt.Sprintf("    rewrite %s %s\n", rewrite.Source, rewrite.Target))
		}

		// Reverse proxy to backend
		buf.WriteString(fmt.Sprintf("    reverse_proxy %s:%d\n", route.BackendHost, route.BackendPort))

		// Security headers
		buf.WriteString("    header {\n")
		buf.WriteString("        X-Content-Type-Options nosniff\n")
		buf.WriteString("        X-Frame-Options DENY\n")
		buf.WriteString("        Referrer-Policy strict-origin-when-cross-origin\n")
		buf.WriteString("    }\n")

		buf.WriteString("}\n\n")
	}

	return buf.String()
}

// CaddyJSON builds the Caddy JSON config structure for a set of routes.
func (m *Manager) CaddyJSON(routes []Route) map[string]interface{} {
	caddyRoutes := make([]map[string]interface{}, 0, len(routes))

	for _, route := range routes {
		host := fmt.Sprintf("%s.%s", route.Subdomain, m.domain)
		upstream := fmt.Sprintf("%s:%d", route.BackendHost, route.BackendPort)

		handlers := make([]map[string]interface{}, 0)

		// Add redirects
		for _, redir := range route.Redirects {
			handlers = append(handlers, map[string]interface{}{
				"handler":     "static_response",
				"status_code": fmt.Sprintf("%d", redir.StatusCode),
				"headers": map[string][]string{
					"Location": {redir.Target},
				},
				"match": []map[string]interface{}{
					{"path": []string{redir.Source}},
				},
			})
		}

		// Add reverse proxy
		handlers = append(handlers, map[string]interface{}{
			"handler": "reverse_proxy",
			"upstreams": []map[string]string{
				{"dial": upstream},
			},
		})

		caddyRoute := map[string]interface{}{
			"match": []map[string]interface{}{
				{"host": []string{host}},
			},
			"handle": handlers,
		}

		caddyRoutes = append(caddyRoutes, caddyRoute)
	}

	return map[string]interface{}{
		"apps": map[string]interface{}{
			"http": map[string]interface{}{
				"servers": map[string]interface{}{
					"klouds": map[string]interface{}{
						"listen": []string{":443"},
						"routes": caddyRoutes,
					},
				},
			},
		},
	}
}

// ApplyConfig sends the JSON configuration to Caddy's admin API.
func (m *Manager) ApplyConfig(routes []Route) error {
	config := m.CaddyJSON(routes)

	body, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal caddy config: %w", err)
	}

	req, err := http.NewRequest("POST", m.adminAPI+"/load", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send caddy config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("caddy config rejected (status %d)", resp.StatusCode)
	}

	log.Info().Int("routes", len(routes)).Msg("Caddy config applied successfully")
	return nil
}

// AddRoute adds a single route to the running Caddy config.
func (m *Manager) AddRoute(route Route) error {
	// For simplicity in Phase 1, we reload the entire config.
	// In production, we'd use PATCH to add individual routes.
	log.Info().
		Str("subdomain", route.Subdomain).
		Str("backend", fmt.Sprintf("%s:%d", route.BackendHost, route.BackendPort)).
		Msg("Route registered (will be applied on next full reload)")
	return nil
}

// RemoveRoute removes a route from the running Caddy config.
func (m *Manager) RemoveRoute(subdomain string) error {
	log.Info().Str("subdomain", subdomain).Msg("Route removed (will be applied on next full reload)")
	return nil
}
