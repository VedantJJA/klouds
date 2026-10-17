package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// --- File Manager Types ---

type FileItem struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"` // relative path
	IsDir      bool      `json:"is_dir"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"mod_time"`
	Extension  string    `json:"extension"`
	IsEditable bool      `json:"is_editable"`
}

type SaveFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type CreateFolderRequest struct {
	Path string `json:"path"` // parent directory
	Name string `json:"name"` // folder name
}

type DeleteFileRequest struct {
	Path string `json:"path"`
}

type InstallModRequest struct {
	DownloadURL string `json:"download_url"`
	Filename    string `json:"filename"`
	TargetDir   string `json:"target_dir"` // "mods" or "plugins"
}

type ToggleModRequest struct {
	Dir      string `json:"dir"` // "mods" or "plugins"
	Filename string `json:"filename"`
}

type DeleteModRequest struct {
	Dir      string `json:"dir"`
	Filename string `json:"filename"`
}

type InstalledModItem struct {
	Name      string `json:"name"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	Enabled   bool   `json:"enabled"`
	Dir       string `json:"dir"`
	ModTime   time.Time `json:"mod_time"`
}

// --- Path Traversal Protection Helper ---

func (h *MinecraftHandler) resolveServerPath(slug, subPath string) (string, error) {
	baseDir := filepath.Join(h.dataDir, "minecraft", slug)
	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to ensure base dir: %w", err)
	}

	cleanSub := filepath.Clean("/" + subPath)
	cleanSub = strings.TrimPrefix(cleanSub, "/")
	cleanSub = strings.TrimPrefix(cleanSub, "\\")

	targetPath := filepath.Join(baseDir, cleanSub)

	// Ensure the target is strictly inside baseDir
	rel, err := filepath.Rel(baseDir, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("access denied: path traversal detected")
	}

	return targetPath, nil
}

// isEditableFile determines whether a file can be opened in the web text editor
func isEditableFile(ext string) bool {
	switch strings.ToLower(ext) {
	case ".txt", ".properties", ".json", ".yml", ".yaml", ".toml", ".cfg", ".conf",
		".log", ".sh", ".env", ".xml", ".ini", ".md", ".csv", ".mcmeta":
		return true
	default:
		return false
	}
}

// --- File Manager Endpoints ---

// ListFiles handles GET /api/minecraft/{id}/files?path=...
func (h *MinecraftHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	reqPath := r.URL.Query().Get("path")
	fullPath, err := h.resolveServerPath(srv.Slug, reqPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []FileItem{})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read directory: "+err.Error())
		return
	}

	items := make([]FileItem, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		itemRelPath := filepath.Join(reqPath, entry.Name())
		// Clean forward slashes for cross-platform web client
		itemRelPath = filepath.ToSlash(itemRelPath)
		ext := filepath.Ext(entry.Name())

		items = append(items, FileItem{
			Name:       entry.Name(),
			Path:       itemRelPath,
			IsDir:      entry.IsDir(),
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			Extension:  ext,
			IsEditable: !entry.IsDir() && isEditableFile(ext),
		})
	}

	// Sort directories first, then alphabetical
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	writeJSON(w, http.StatusOK, items)
}

// GetFileContent handles GET /api/minecraft/{id}/files/content?path=...
func (h *MinecraftHandler) GetFileContent(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		writeError(w, http.StatusBadRequest, "path parameter required")
		return
	}

	fullPath, err := h.resolveServerPath(srv.Slug, reqPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "cannot read directory as file")
		return
	}

	// 3MB limit for web editor
	if info.Size() > 3*1024*1024 {
		writeError(w, http.StatusBadRequest, "file too large for text editor (max 3MB)")
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":       filepath.Base(fullPath),
		"path":       filepath.ToSlash(reqPath),
		"content":    string(content),
		"size":       info.Size(),
		"mod_time":   info.ModTime(),
		"extension":  filepath.Ext(fullPath),
		"is_editable": isEditableFile(filepath.Ext(fullPath)),
	})
}

// SaveFileContent handles PUT /api/minecraft/{id}/files/content
func (h *MinecraftHandler) SaveFileContent(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	var req SaveFileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	fullPath, err := h.resolveServerPath(srv.Slug, req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create parent directories")
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write file: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "saved",
		"path":   filepath.ToSlash(req.Path),
	})
}

// CreateFolder handles POST /api/minecraft/{id}/files/folder
func (h *MinecraftHandler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	var req CreateFolderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		writeError(w, http.StatusBadRequest, "valid folder name is required")
		return
	}

	folderRelPath := filepath.Join(req.Path, name)
	fullPath, err := h.resolveServerPath(srv.Slug, folderRelPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create folder: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"status": "created",
		"path":   filepath.ToSlash(folderRelPath),
	})
}

// DeleteFile handles POST /api/minecraft/{id}/files/delete
func (h *MinecraftHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	var req DeleteFileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Path) == "" || req.Path == "/" || req.Path == "." {
		writeError(w, http.StatusBadRequest, "cannot delete root directory")
		return
	}

	fullPath, err := h.resolveServerPath(srv.Slug, req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	baseDir := filepath.Join(h.dataDir, "minecraft", srv.Slug)
	if fullPath == baseDir {
		writeError(w, http.StatusBadRequest, "cannot delete root directory")
		return
	}

	if err := os.RemoveAll(fullPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// UploadFiles handles POST /api/minecraft/{id}/files/upload
func (h *MinecraftHandler) UploadFiles(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	// 500MB max upload size (covers large modpacks or maps)
	if err := r.ParseMultipartForm(500 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse upload: "+err.Error())
		return
	}

	destPath := r.FormValue("path")
	targetDir, err := h.resolveServerPath(srv.Slug, destPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create destination dir")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		// Also check singular "file" field
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no files uploaded")
		return
	}

	uploadedNames := []string{}
	for _, fileHeader := range files {
		cleanName := filepath.Base(fileHeader.Filename)
		if cleanName == "" || cleanName == "." || cleanName == ".." {
			continue
		}

		dstFilePath := filepath.Join(targetDir, cleanName)
		dstFile, err := os.OpenFile(dstFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Warn().Err(err).Str("file", cleanName).Msg("Failed to open destination file for upload")
			continue
		}

		srcFile, err := fileHeader.Open()
		if err != nil {
			dstFile.Close()
			continue
		}

		_, copyErr := io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()

		if copyErr == nil {
			uploadedNames = append(uploadedNames, cleanName)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "uploaded",
		"uploaded": uploadedNames,
		"count":    len(uploadedNames),
	})
}

// DownloadFile handles GET /api/minecraft/{id}/files/download?path=...
func (h *MinecraftHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	reqPath := r.URL.Query().Get("path")
	fullPath, err := h.resolveServerPath(srv.Slug, reqPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	filename := filepath.Base(fullPath)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	http.ServeFile(w, r, fullPath)
}

// --- Mods & Plugins Management Endpoints ---

// ListMods handles GET /api/minecraft/{id}/mods
func (h *MinecraftHandler) ListMods(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	scanDir := func(subDir string) []InstalledModItem {
		res := []InstalledModItem{}
		fullDir, err := h.resolveServerPath(srv.Slug, subDir)
		if err != nil {
			return res
		}

		entries, err := os.ReadDir(fullDir)
		if err != nil {
			return res
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			info, err := e.Info()
			if err != nil {
				continue
			}

			isJar := strings.HasSuffix(strings.ToLower(name), ".jar")
			isDisabledJar := strings.HasSuffix(strings.ToLower(name), ".jar.disabled")

			if isJar || isDisabledJar {
				cleanDisplayName := strings.TrimSuffix(name, ".disabled")
				cleanDisplayName = strings.TrimSuffix(cleanDisplayName, ".jar")

				res = append(res, InstalledModItem{
					Name:     cleanDisplayName,
					Filename: name,
					Size:     info.Size(),
					Enabled:  isJar,
					Dir:      subDir,
					ModTime:  info.ModTime(),
				})
			}
		}

		sort.Slice(res, func(i, j int) bool {
			return strings.ToLower(res[i].Name) < strings.ToLower(res[j].Name)
		})
		return res
	}

	mods := scanDir("mods")
	plugins := scanDir("plugins")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mods":    mods,
		"plugins": plugins,
	})
}

// InstallMod handles POST /api/minecraft/{id}/mods/install
func (h *MinecraftHandler) InstallMod(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	var req InstallModRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.DownloadURL == "" || req.Filename == "" {
		writeError(w, http.StatusBadRequest, "download_url and filename are required")
		return
	}

	targetDir := "mods"
	if strings.ToLower(req.TargetDir) == "plugins" {
		targetDir = "plugins"
	}

	cleanFilename := filepath.Base(req.Filename)
	if !strings.HasSuffix(strings.ToLower(cleanFilename), ".jar") {
		cleanFilename += ".jar"
	}

	destPath, err := h.resolveServerPath(srv.Slug, filepath.Join(targetDir, cleanFilename))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Ensure destination directory exists
	_ = os.MkdirAll(filepath.Dir(destPath), 0755)

	// Download file from URL (e.g. Modrinth CDN)
	client := &http.Client{Timeout: 60 * time.Second}
	dlReq, err := http.NewRequestWithContext(r.Context(), "GET", req.DownloadURL, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid download url")
		return
	}
	dlReq.Header.Set("User-Agent", "Klouds-Minecraft-Manager/1.0 (contact@klouds.online)")

	resp, err := client.Do(dlReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to download mod from remote host: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("remote download returned HTTP %d", resp.StatusCode))
		return
	}

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file to server: "+err.Error())
		return
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed writing mod to disk: "+err.Error())
		return
	}

	log.Info().
		Str("server", srv.Slug).
		Str("file", cleanFilename).
		Int64("bytes", written).
		Msg("Successfully installed Minecraft mod/plugin")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "installed",
		"filename": cleanFilename,
		"size":     written,
		"dir":      targetDir,
	})
}

// ToggleMod handles POST /api/minecraft/{id}/mods/toggle
func (h *MinecraftHandler) ToggleMod(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	var req ToggleModRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	targetDir := "mods"
	if strings.ToLower(req.Dir) == "plugins" {
		targetDir = "plugins"
	}

	cleanFilename := filepath.Base(req.Filename)
	curPath, err := h.resolveServerPath(srv.Slug, filepath.Join(targetDir, cleanFilename))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var newFilename string
	if strings.HasSuffix(cleanFilename, ".jar.disabled") {
		newFilename = strings.TrimSuffix(cleanFilename, ".disabled")
	} else if strings.HasSuffix(cleanFilename, ".jar") {
		newFilename = cleanFilename + ".disabled"
	} else {
		writeError(w, http.StatusBadRequest, "only .jar files can be toggled")
		return
	}

	newPath, err := h.resolveServerPath(srv.Slug, filepath.Join(targetDir, newFilename))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := os.Rename(curPath, newPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to toggle mod status: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "toggled",
		"new_filename": newFilename,
		"enabled":      strings.HasSuffix(newFilename, ".jar"),
	})
}

// DeleteMod handles DELETE /api/minecraft/{id}/mods
func (h *MinecraftHandler) DeleteMod(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	dir := r.URL.Query().Get("dir")
	filename := r.URL.Query().Get("filename")
	if dir == "" || filename == "" {
		writeError(w, http.StatusBadRequest, "dir and filename query parameters required")
		return
	}

	targetDir := "mods"
	if strings.ToLower(dir) == "plugins" {
		targetDir = "plugins"
	}

	cleanFilename := filepath.Base(filename)
	filePath, err := h.resolveServerPath(srv.Slug, filepath.Join(targetDir, cleanFilename))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := os.Remove(filePath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete mod file: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "deleted",
		"filename": cleanFilename,
	})
}

// --- Dynamic Minecraft Versions & Modrinth Proxies ---

var (
	cachedVersions     []string
	cachedLatestRel    string
	versionsCacheMutex sync.RWMutex
	versionsLastFetch  time.Time
)

// Fallback versions in case network is down
var defaultFallbackVersions = []string{
	"1.21.4", "1.21.3", "1.21.2", "1.21.1", "1.21",
	"1.20.6", "1.20.4", "1.20.2", "1.20.1", "1.20",
	"1.19.4", "1.19.3", "1.19.2", "1.19",
	"1.18.2", "1.18.1", "1.17.1", "1.16.5", "1.12.2", "1.8.8",
}

type mojangVersionManifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"versions"`
}

// GetVersions handles GET /api/minecraft/versions
func (h *MinecraftHandler) GetVersions(w http.ResponseWriter, r *http.Request) {
	versionsCacheMutex.RLock()
	cached := cachedVersions
	latest := cachedLatestRel
	lastFetch := versionsLastFetch
	versionsCacheMutex.RUnlock()

	// If cache is fresh (< 30 minutes), return it
	if len(cached) > 0 && time.Since(lastFetch) < 30*time.Minute {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"latest":   latest,
			"versions": cached,
		})
		return
	}

	// Fetch from Mojang official version manifest
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get("https://launchermeta.mojang.com/mc/game/version_manifest_v2.json")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch Minecraft version manifest from Mojang, using fallback")
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"latest":   "1.21.4",
			"versions": defaultFallbackVersions,
		})
		return
	}
	defer resp.Body.Close()

	var manifest mojangVersionManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"latest":   "1.21.4",
			"versions": defaultFallbackVersions,
		})
		return
	}

	releases := make([]string, 0, len(manifest.Versions))
	for _, v := range manifest.Versions {
		if v.Type == "release" {
			releases = append(releases, v.ID)
		}
	}

	latestRel := manifest.Latest.Release
	if latestRel == "" && len(releases) > 0 {
		latestRel = releases[0]
	}

	versionsCacheMutex.Lock()
	cachedVersions = releases
	cachedLatestRel = latestRel
	versionsLastFetch = time.Now()
	versionsCacheMutex.Unlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"latest":   latestRel,
		"versions": releases,
	})
}

// SearchModrinth handles GET /api/minecraft/modrinth/search
func (h *MinecraftHandler) SearchModrinth(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	loader := strings.ToLower(r.URL.Query().Get("loader"))
	version := r.URL.Query().Get("version")
	projType := r.URL.Query().Get("project_type") // "mod" or "plugin"
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "20"
	}
	offset := r.URL.Query().Get("offset")
	if offset == "" {
		offset = "0"
	}

	// Build facets array for Modrinth API v2
	// Example: [["project_type:mod"],["categories:fabric"],["versions:1.21.4"]]
	var facetGroups []string
	if projType != "" {
		facetGroups = append(facetGroups, fmt.Sprintf(`["project_type:%s"]`, url.QueryEscape(projType)))
	}
	if loader != "" && loader != "vanilla" {
		facetGroups = append(facetGroups, fmt.Sprintf(`["categories:%s"]`, url.QueryEscape(loader)))
	}
	if version != "" && version != "LATEST" {
		facetGroups = append(facetGroups, fmt.Sprintf(`["versions:%s"]`, url.QueryEscape(version)))
	}

	facetsParam := ""
	if len(facetGroups) > 0 {
		facetsParam = "[" + strings.Join(facetGroups, ",") + "]"
	}

	modrinthURL := fmt.Sprintf("https://api.modrinth.com/v2/search?limit=%s&offset=%s", limit, offset)
	if query != "" {
		modrinthURL += "&query=" + url.QueryEscape(query)
	}
	if facetsParam != "" {
		modrinthURL += "&facets=" + url.QueryEscape(facetsParam)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), "GET", modrinthURL, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid search request")
		return
	}
	req.Header.Set("User-Agent", "Klouds-PaaS/1.0 (contact@klouds.online)")

	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to query Modrinth API: "+err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// GetModrinthVersions handles GET /api/minecraft/modrinth/versions?project_id=...&loader=...&version=...
func (h *MinecraftHandler) GetModrinthVersions(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	loader := strings.ToLower(r.URL.Query().Get("loader"))
	gameVer := r.URL.Query().Get("version")

	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project_id parameter required")
		return
	}

	modrinthURL := fmt.Sprintf("https://api.modrinth.com/v2/project/%s/version", url.PathEscape(projectID))
	queryParams := url.Values{}
	if loader != "" && loader != "vanilla" {
		queryParams.Set("loaders", fmt.Sprintf(`["%s"]`, loader))
	}
	if gameVer != "" && gameVer != "LATEST" {
		queryParams.Set("game_versions", fmt.Sprintf(`["%s"]`, gameVer))
	}
	if len(queryParams) > 0 {
		modrinthURL += "?" + queryParams.Encode()
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), "GET", modrinthURL, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	req.Header.Set("User-Agent", "Klouds-PaaS/1.0 (contact@klouds.online)")

	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch project versions: "+err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
