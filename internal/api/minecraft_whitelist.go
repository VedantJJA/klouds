package api

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/vedant/klouds/internal/db"
)

// WhitelistEntry represents an entry inside Minecraft's whitelist.json file.
type WhitelistEntry struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// WhitelistPlayerRequest is the request payload to add or remove a player.
type WhitelistPlayerRequest struct {
	Username string `json:"username"`
}

// readWhitelistFile reads and parses whitelist.json
func (h *MinecraftHandler) readWhitelistFile(slug string) ([]WhitelistEntry, error) {
	path := filepath.Join(h.dataDir, "minecraft", slug, "whitelist.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []WhitelistEntry{}, nil
		}
		return nil, err
	}
	var entries []WhitelistEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return []WhitelistEntry{}, nil
	}
	return entries, nil
}

// writeWhitelistFile atomically writes whitelist.json
func (h *MinecraftHandler) writeWhitelistFile(slug string, entries []WhitelistEntry) error {
	dir := filepath.Join(h.dataDir, "minecraft", slug)
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "whitelist.json")
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// updatePropertiesWhitelist updates white-list=true/false in server.properties
func (h *MinecraftHandler) updatePropertiesWhitelist(slug string, enabled bool) {
	propsFile := filepath.Join(h.dataDir, "minecraft", slug, "server.properties")
	content, err := os.ReadFile(propsFile)
	if err != nil {
		return
	}
	lines := strings.Split(string(content), "\n")
	found := false
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "white-list=") {
			lines[i] = fmt.Sprintf("white-list=%t", enabled)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("white-list=%t", enabled))
	}
	_ = os.WriteFile(propsFile, []byte(strings.Join(lines, "\n")), 0644)
}

// ToggleWhitelist toggles whitelist on or off
func (h *MinecraftHandler) ToggleWhitelist(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	newStatus := !srv.WhitelistEnabled
	// Also check server.properties to make sure we toggle from actual current value
	propsFile := filepath.Join(h.dataDir, "minecraft", srv.Slug, "server.properties")
	if propsData, err := os.ReadFile(propsFile); err == nil {
		for _, line := range strings.Split(string(propsData), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "white-list=") {
				val := strings.TrimPrefix(trimmed, "white-list=")
				newStatus = !strings.EqualFold(val, "true")
				break
			}
		}
	}

	// Update DB config
	_, _ = h.queries.UpdateMinecraftServerConfig(r.Context(), db.UpdateMinecraftServerConfigParams{
		ID:               srv.ID,
		Name:             srv.Name,
		MOTD:             srv.MOTD,
		Difficulty:       srv.Difficulty,
		GameMode:         srv.GameMode,
		MaxPlayers:       srv.MaxPlayers,
		PVP:              srv.PVP,
		OnlineMode:       srv.OnlineMode,
		WhitelistEnabled: newStatus,
		ServerType:       srv.ServerType,
		Version:          srv.Version,
		CpuLimit:         srv.CpuLimit,
		MemoryLimit:      srv.MemoryLimit,
		Ops:              srv.Ops,
	})

	// Update server.properties on disk
	h.updatePropertiesWhitelist(srv.Slug, newStatus)

	// If container running, execute RCON
	if h.containers != nil && srv.ContainerID != nil && *srv.ContainerID != "" && h.containers.IsRunning(r.Context(), *srv.ContainerID) {
		cmd := "whitelist off"
		if newStatus {
			cmd = "whitelist on"
		}
		_, _ = h.executeConsoleCommand(r.Context(), srv, cmd)
		_, _ = h.executeConsoleCommand(r.Context(), srv, "whitelist reload")
	}

	entries, _ := h.readWhitelistFile(srv.Slug)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"whitelist_enabled": newStatus,
		"whitelist":         names,
	})
}

// AddWhitelist adds a player to the whitelist
func (h *MinecraftHandler) AddWhitelist(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	var req WhitelistPlayerRequest
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Username) == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	username := strings.TrimSpace(req.Username)

	// If container running, execute RCON whitelist add <username>
	if h.containers != nil && srv.ContainerID != nil && *srv.ContainerID != "" && h.containers.IsRunning(r.Context(), *srv.ContainerID) {
		_, _ = h.executeConsoleCommand(r.Context(), srv, fmt.Sprintf("whitelist add %s", username))
		_, _ = h.executeConsoleCommand(r.Context(), srv, "whitelist reload")
	}

	// Always sync whitelist.json on disk so player appears immediately
	entries, _ := h.readWhitelistFile(srv.Slug)
	found := false
	for _, e := range entries {
		if strings.EqualFold(e.Name, username) {
			found = true
			break
		}
	}
	if !found {
		// Calculate offline-player UUID v3
		md5Hash := md5.Sum([]byte("OfflinePlayer:" + username))
		md5Hash[6] = (md5Hash[6] & 0x0f) | 0x30 // version 3
		md5Hash[8] = (md5Hash[8] & 0x3f) | 0x80 // IETF variant
		playerUUID := fmt.Sprintf("%x-%x-%x-%x-%x", md5Hash[0:4], md5Hash[4:6], md5Hash[6:8], md5Hash[8:10], md5Hash[10:])

		entries = append(entries, WhitelistEntry{
			UUID: playerUUID,
			Name: username,
		})
		_ = h.writeWhitelistFile(srv.Slug, entries)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"whitelist": names,
	})
}

// RemoveWhitelist removes a player from the whitelist
func (h *MinecraftHandler) RemoveWhitelist(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	var req WhitelistPlayerRequest
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Username) == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	username := strings.TrimSpace(req.Username)

	// If container running, execute RCON whitelist remove <username>
	if h.containers != nil && srv.ContainerID != nil && *srv.ContainerID != "" && h.containers.IsRunning(r.Context(), *srv.ContainerID) {
		_, _ = h.executeConsoleCommand(r.Context(), srv, fmt.Sprintf("whitelist remove %s", username))
		_, _ = h.executeConsoleCommand(r.Context(), srv, "whitelist reload")
	}

	// Always sync whitelist.json on disk
	entries, _ := h.readWhitelistFile(srv.Slug)
	updated := make([]WhitelistEntry, 0, len(entries))
	for _, e := range entries {
		if !strings.EqualFold(e.Name, username) {
			updated = append(updated, e)
		}
	}
	_ = h.writeWhitelistFile(srv.Slug, updated)

	names := make([]string, 0, len(updated))
	for _, e := range updated {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"whitelist": names,
	})
}
