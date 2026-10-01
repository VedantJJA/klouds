// Package caddy generates and applies Caddy reverse proxy configuration
// via the Caddy Admin API (http://localhost:2019).
package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

// OrderedRule represents an ordered redirect or rewrite rule (supporting internal paths and external URLs).
type OrderedRule struct {
	Type       string // "redirect" or "rewrite"
	Source     string // path pattern, e.g. "/*" or "/api/*"
	Target     string // destination path or external URL, e.g. "/index.html" or "https://api.external.com"
	StatusCode int    // 301 or 302 for redirects
}

// Route represents a Caddy route to a backend container.
type Route struct {
	Subdomain   string // e.g. "myapp" -> myapp.domain.com
	BackendHost string // Docker network hostname
	BackendPort int    // Container internal port
	Rules       []OrderedRule
	Redirects   []RedirectRule // Backwards compatibility
	Rewrites    []RewriteRule  // Backwards compatibility
}

// RedirectRule represents a redirect configuration.
type RedirectRule struct {
	Source     string
	Target     string
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
	buf.WriteString("    on_demand_tls {\n")
	buf.WriteString("        ask http://127.0.0.1:8080/api/tls/check\n")
	buf.WriteString("        interval 2m\n")
	buf.WriteString("        burst 5\n")
	buf.WriteString("    }\n")
	buf.WriteString("}\n\n")

	for _, route := range routes {
		host := fmt.Sprintf("%s.%s", route.Subdomain, m.domain)
		buf.WriteString(fmt.Sprintf("%s {\n", host))
		buf.WriteString("    tls {\n        on_demand\n    }\n")

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
		handlers := buildRouteHandlers(upstream, route.Rules, route.Redirects, route.Rewrites)

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
			"tls": map[string]interface{}{
				"automation": map[string]interface{}{
					"policies": []map[string]interface{}{
						{
							"on_demand": true,
						},
					},
					"on_demand": map[string]interface{}{
						"ask": "http://127.0.0.1:8080/api/tls/check",
					},
				},
			},
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
	host := fmt.Sprintf("%s.%s", route.Subdomain, m.domain)
	upstream := fmt.Sprintf("%s:%d", route.BackendHost, route.BackendPort)

	routeID := fmt.Sprintf("route-%s", route.Subdomain)
	handlers := buildRouteHandlers(upstream, route.Rules, route.Redirects, route.Rewrites)

	caddyRoute := map[string]interface{}{
		"@id":      routeID,
		"match":    []map[string]interface{}{{"host": []string{host}}},
		"handle":   handlers,
		"terminal": true,
	}

	body, err := json.Marshal(caddyRoute)
	if err != nil {
		return err
	}

	// Remove previous route with this @id if present, so we can cleanly insert at index 0
	_ = m.RemoveRoute(route.Subdomain)

	// Insert at index 0 of srv0 routes (before the catch-all wildcard *.domain)
	req2, err := http.NewRequest("PUT", fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/0", m.adminAPI), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create caddy route request: %w", err)
	}
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return fmt.Errorf("send caddy route request: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode >= 400 {
		return fmt.Errorf("caddy route insertion returned status %d", resp2.StatusCode)
	}

	log.Info().Str("host", host).Str("upstream", upstream).Msg("Caddy dynamic route added successfully")
	return nil
}

// RemoveRoute removes a route from the running Caddy config.
func (m *Manager) RemoveRoute(subdomain string) error {
	routeID := fmt.Sprintf("route-%s", subdomain)
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/id/%s", m.adminAPI, routeID), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Info().Str("subdomain", subdomain).Msg("Caddy dynamic route removed")
	return nil
}

// PruneOrphanRoutes queries all dynamic routes in Caddy and removes any whose @id starts with "route-"
// but whose subdomain is not present in activeSubdomains.
func (m *Manager) PruneOrphanRoutes(activeSubdomains []string) ([]string, error) {
	activeMap := make(map[string]bool)
	for _, sub := range activeSubdomains {
		sub = strings.TrimSpace(strings.ToLower(sub))
		if sub != "" {
			activeMap[sub] = true
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/config/apps/http/servers/srv0/routes", m.adminAPI), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caddy routes returned status %d", resp.StatusCode)
	}

	var routes []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return nil, fmt.Errorf("decode caddy routes: %w", err)
	}

	var pruned []string
	for _, route := range routes {
		id, ok := route["@id"].(string)
		if !ok || !strings.HasPrefix(id, "route-") {
			continue
		}
		subdomain := strings.TrimPrefix(id, "route-")
		if !activeMap[strings.ToLower(subdomain)] {
			log.Info().Str("subdomain", subdomain).Msg("Pruning unreferenced Caddy route")
			if err := m.RemoveRoute(subdomain); err != nil {
				log.Warn().Err(err).Str("subdomain", subdomain).Msg("Failed to remove unreferenced Caddy route")
			} else {
				pruned = append(pruned, subdomain)
			}
		}
	}

	return pruned, nil
}

// buildRouteHandlers creates the Caddy handler chain for a route, maintaining strict evaluation order and supporting external rewrites.
func buildRouteHandlers(upstream string, orderedRules []OrderedRule, redirects []RedirectRule, rewrites []RewriteRule) []map[string]interface{} {
	// Consolidate into unified ordered list if legacy slices are used
	rules := make([]OrderedRule, 0, len(orderedRules)+len(redirects)+len(rewrites))
	if len(orderedRules) > 0 {
		rules = append(rules, orderedRules...)
	} else {
		for _, r := range redirects {
			rules = append(rules, OrderedRule{
				Type:       "redirect",
				Source:     r.Source,
				Target:     r.Target,
				StatusCode: r.StatusCode,
			})
		}
		for _, rw := range rewrites {
			rules = append(rules, OrderedRule{
				Type:   "rewrite",
				Source: rw.Source,
				Target: rw.Target,
			})
		}
	}

	if len(rules) == 0 {
		return []map[string]interface{}{
			{
				"handler":   "reverse_proxy",
				"upstreams": []map[string]string{{"dial": upstream}},
			},
		}
	}

	subroutes := make([]map[string]interface{}, 0, len(rules)+1)

	// Process rules in exact sequential order (Render-style top-to-bottom first-match-wins)
	for _, rule := range rules {
		src := strings.TrimSpace(rule.Source)
		dst := strings.TrimSpace(rule.Target)
		if src == "" || dst == "" {
			continue
		}

		if rule.Type == "redirect" {
			status := rule.StatusCode
			if status != 301 && status != 302 {
				status = 301
			}
			subroutes = append(subroutes, map[string]interface{}{
				"match": []map[string]interface{}{
					{"path": []string{src}},
				},
				"handle": []map[string]interface{}{
					{
						"handler":     "static_response",
						"status_code": fmt.Sprintf("%d", status),
						"headers": map[string][]string{
							"Location": {dst},
						},
					},
				},
			})
		} else {
			// Rewrite rule: check if destination is an external URL (e.g. https://api.external.com)
			if strings.HasPrefix(dst, "http://") || strings.HasPrefix(dst, "https://") {
				u, err := url.Parse(dst)
				if err == nil && u.Host != "" {
					dialTarget := u.Host
					if !strings.Contains(dialTarget, ":") {
						if u.Scheme == "https" {
							dialTarget += ":443"
						} else {
							dialTarget += ":80"
						}
					}

					proxyHandler := map[string]interface{}{
						"handler":   "reverse_proxy",
						"upstreams": []map[string]string{{"dial": dialTarget}},
						"headers": map[string]interface{}{
							"request": map[string]interface{}{
								"set": map[string][]string{
									"Host": {u.Hostname()},
								},
							},
						},
					}

					if u.Scheme == "https" {
						proxyHandler["transport"] = map[string]interface{}{
							"protocol": "http",
							"tls": map[string]interface{}{
								"server_name": u.Hostname(),
							},
						}
					}

					subroutes = append(subroutes, map[string]interface{}{
						"match": []map[string]interface{}{
							{"path": []string{src}},
						},
						"handle": []map[string]interface{}{
							proxyHandler,
						},
					})
					continue
				}
			}

			// Internal path rewrite
			subroutes = append(subroutes, map[string]interface{}{
				"match": []map[string]interface{}{
					{"path": []string{src}},
				},
				"handle": []map[string]interface{}{
					{
						"handler": "rewrite",
						"uri":     dst,
					},
				},
			})
		}
	}

	// Fallback to primary container upstream
	subroutes = append(subroutes, map[string]interface{}{
		"handle": []map[string]interface{}{
			{
				"handler":   "reverse_proxy",
				"upstreams": []map[string]string{{"dial": upstream}},
			},
		},
	})

	return []map[string]interface{}{
		{
			"handler": "subroute",
			"routes":  subroutes,
		},
	}
}
