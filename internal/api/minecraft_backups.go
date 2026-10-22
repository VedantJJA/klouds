package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// BackupItem represents a single Minecraft world backup archive.
type BackupItem struct {
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	SizeFormatted string    `json:"size_formatted"`
	CreatedAt     time.Time `json:"created_at"`
}

// RestoreBackupRequest is the request payload for restoring a world backup.
type RestoreBackupRequest struct {
	Name string `json:"name"`
}

// GDriveUploadRequest is the request payload for uploading a backup to Google Drive.
type GDriveUploadRequest struct {
	Filename    string `json:"filename"`
	AccessToken string `json:"access_token"`
}

// formatBytes returns human-readable bytes (e.g. "64.2 MB")
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ListBackups handles GET /api/minecraft/{id}/backups
func (h *MinecraftHandler) ListBackups(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	backupsDir := filepath.Join(h.dataDir, "minecraft", srv.Slug, "backups")
	_ = os.MkdirAll(backupsDir, 0755)

	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read backups directory")
		return
	}

	var items []BackupItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".zip") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		items = append(items, BackupItem{
			Name:          name,
			Size:          info.Size(),
			SizeFormatted: formatBytes(info.Size()),
			CreatedAt:     info.ModTime(),
		})
	}

	// Sort newest first
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"backups": items,
	})
}

// CreateBackup handles POST /api/minecraft/{id}/backups
func (h *MinecraftHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	serverDir := filepath.Join(h.dataDir, "minecraft", srv.Slug)
	backupsDir := filepath.Join(serverDir, "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create backups folder")
		return
	}

	isRunning := h.containers != nil && srv.ContainerID != nil && *srv.ContainerID != "" && h.containers.IsRunning(r.Context(), *srv.ContainerID)

	// If server running, flush and freeze world saves safely
	if isRunning {
		_, _ = h.executeConsoleCommand(r.Context(), srv, "save-off")
		_, _ = h.executeConsoleCommand(r.Context(), srv, "save-all flush")
		time.Sleep(500 * time.Millisecond)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupFileName := fmt.Sprintf("%s-world-%s.tar.gz", srv.Slug, timestamp)
	backupFilePath := filepath.Join(backupsDir, backupFileName)

	err = createWorldArchive(serverDir, backupFilePath)

	// Re-enable autosave
	if isRunning {
		_, _ = h.executeConsoleCommand(r.Context(), srv, "save-on")
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to create world backup archive")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create world backup: %v", err))
		return
	}

	fileInfo, err := os.Stat(backupFilePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup created but stat failed")
		return
	}

	item := BackupItem{
		Name:          backupFileName,
		Size:          fileInfo.Size(),
		SizeFormatted: formatBytes(fileInfo.Size()),
		CreatedAt:     fileInfo.ModTime(),
	}

	writeJSON(w, http.StatusCreated, item)
}

// DownloadBackup handles GET /api/minecraft/{id}/backups/download?name=...
func (h *MinecraftHandler) DownloadBackup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	fileName := filepath.Base(r.URL.Query().Get("name"))
	if fileName == "" || fileName == "." || fileName == "/" {
		writeError(w, http.StatusBadRequest, "invalid backup name")
		return
	}

	filePath := filepath.Join(h.dataDir, "minecraft", srv.Slug, "backups", fileName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "backup file not found")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	w.Header().Set("Content-Type", "application/gzip")
	http.ServeFile(w, r, filePath)
}

// DeleteBackup handles DELETE /api/minecraft/{id}/backups?name=...
func (h *MinecraftHandler) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	fileName := filepath.Base(r.URL.Query().Get("name"))
	if fileName == "" || fileName == "." || fileName == "/" {
		writeError(w, http.StatusBadRequest, "invalid backup name")
		return
	}

	filePath := filepath.Join(h.dataDir, "minecraft", srv.Slug, "backups", fileName)
	if err := os.Remove(filePath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete backup")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// RestoreBackup handles POST /api/minecraft/{id}/backups/restore
func (h *MinecraftHandler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	var req RestoreBackupRequest
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "backup name is required")
		return
	}

	isRunning := h.containers != nil && srv.ContainerID != nil && *srv.ContainerID != "" && h.containers.IsRunning(r.Context(), *srv.ContainerID)
	if isRunning {
		writeError(w, http.StatusBadRequest, "Cannot restore world while server is running. Please stop the server first.")
		return
	}

	fileName := filepath.Base(req.Name)
	backupPath := filepath.Join(h.dataDir, "minecraft", srv.Slug, "backups", fileName)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "backup file not found")
		return
	}

	serverDir := filepath.Join(h.dataDir, "minecraft", srv.Slug)

	// Create safety snapshot of current worlds before overwriting
	safetyDir := filepath.Join(serverDir, "backups", "safety")
	_ = os.MkdirAll(safetyDir, 0755)
	_ = createWorldArchive(serverDir, filepath.Join(safetyDir, fmt.Sprintf("pre-restore-%s.tar.gz", time.Now().Format("20060102-150405"))))

	// Extract the backup archive over the server directory
	if err := extractTarGz(backupPath, serverDir); err != nil {
		log.Error().Err(err).Msg("Failed to extract world backup archive")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("restore failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}

// UploadBackupToGDrive handles POST /api/minecraft/{id}/backups/gdrive
func (h *MinecraftHandler) UploadBackupToGDrive(w http.ResponseWriter, r *http.Request) {
	serverID := chi.URLParam(r, "id")
	srv, err := h.getAndVerifyServer(r, serverID)
	if err != nil {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	var req GDriveUploadRequest
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Filename) == "" || strings.TrimSpace(req.AccessToken) == "" {
		writeError(w, http.StatusBadRequest, "filename and access_token are required")
		return
	}

	fileName := filepath.Base(req.Filename)
	backupFilePath := filepath.Join(h.dataDir, "minecraft", srv.Slug, "backups", fileName)
	fileInfo, err := os.Stat(backupFilePath)
	if os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "backup file not found on disk")
		return
	}

	token := strings.TrimSpace(req.AccessToken)

	// 1. Locate or create folder "Klouds Minecraft Backups" in user's Google Drive
	folderID, err := getOrCreateGDriveFolder(token, "Klouds Minecraft Backups")
	if err != nil {
		log.Error().Err(err).Msg("Google Drive folder lookup/create failed")
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Google Drive auth error: %v", err))
		return
	}

	// 2. Upload file directly to Google Drive via multipart upload
	uploadedFile, err := uploadFileToGDrive(token, folderID, backupFilePath, fileName, fileInfo.Size())
	if err != nil {
		log.Error().Err(err).Msg("Google Drive file upload failed")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Google Drive upload failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"file_id":       uploadedFile.ID,
		"file_name":     uploadedFile.Name,
		"web_view_link": fmt.Sprintf("https://drive.google.com/file/d/%s/view", uploadedFile.ID),
	})
}

// --- World Archiving Helpers ---

func createWorldArchive(serverDir string, targetArchive string) error {
	out, err := os.Create(targetArchive)
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Folders and config files to include in world backup
	entries, err := os.ReadDir(serverDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()
		// Target world directories (world, world_nether, world_the_end, etc.) or config files
		isWorldDir := entry.IsDir() && (strings.HasPrefix(strings.ToLower(name), "world") || strings.EqualFold(name, "dim-1") || strings.EqualFold(name, "dim1"))
		isConfigFile := !entry.IsDir() && (name == "server.properties" || name == "whitelist.json" || name == "ops.json")

		if !isWorldDir && !isConfigFile {
			continue
		}

		fullPath := filepath.Join(serverDir, name)
		if entry.IsDir() {
			err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				relPath, err := filepath.Rel(serverDir, path)
				if err != nil {
					return err
				}
				header, err := tar.FileInfoHeader(info, info.Name())
				if err != nil {
					return err
				}
				// Use forward slashes inside tar header
				header.Name = filepath.ToSlash(relPath)

				if err := tw.WriteHeader(header); err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()
				_, err = io.Copy(tw, file)
				return err
			})
			if err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				continue
			}
			header.Name = name
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			file, err := os.Open(fullPath)
			if err != nil {
				continue
			}
			_, _ = io.Copy(tw, file)
			file.Close()
		}
	}

	return nil
}

func extractTarGz(archivePath string, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue // prevent zip slip
		}

		target := filepath.Join(destDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}

// --- Google Drive API Integration Helpers ---

type gdriveFolderList struct {
	Files []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"files"`
}

type gdriveFileResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func getOrCreateGDriveFolder(token string, folderName string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Search for existing folder
	query := fmt.Sprintf("name='%s' and mimeType='application/vnd.google-apps.folder' and trashed=false", folderName)
	reqURL := fmt.Sprintf("https://www.googleapis.com/drive/v3/files?q=%s&fields=files(id,name)", strings.ReplaceAll(query, " ", "%20"))

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("invalid or expired Google Drive access token")
	}

	var list gdriveFolderList
	if err := json.NewDecoder(res.Body).Decode(&list); err == nil && len(list.Files) > 0 {
		return list.Files[0].ID, nil
	}

	// Create new folder
	createBody, _ := json.Marshal(map[string]interface{}{
		"name":     folderName,
		"mimeType": "application/vnd.google-apps.folder",
	})
	createReq, _ := http.NewRequest("POST", "https://www.googleapis.com/drive/v3/files", bytes.NewBuffer(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")

	cRes, err := client.Do(createReq)
	if err != nil {
		return "", err
	}
	defer cRes.Body.Close()

	if cRes.StatusCode < 200 || cRes.StatusCode >= 300 {
		respBytes, _ := io.ReadAll(cRes.Body)
		return "", fmt.Errorf("create folder error (%d): %s", cRes.StatusCode, string(respBytes))
	}

	var created gdriveFileResponse
	if err := json.NewDecoder(cRes.Body).Decode(&created); err != nil {
		return "", err
	}
	return created.ID, nil
}

func uploadFileToGDrive(token, folderID, filePath, fileName string, fileSize int64) (*gdriveFileResponse, error) {
	client := &http.Client{Timeout: 10 * time.Minute}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Part 1: JSON metadata
	metaHeader := make(textproto.MIMEHeader)
	metaHeader.Set("Content-Type", "application/json; charset=UTF-8")
	metaPart, err := writer.CreatePart(metaHeader)
	if err != nil {
		return nil, err
	}
	metaJSON, _ := json.Marshal(map[string]interface{}{
		"name":    fileName,
		"parents": []string{folderID},
	})
	metaPart.Write(metaJSON)

	// Part 2: File content
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Type", "application/gzip")
	mediaPart, err := writer.CreatePart(fileHeader)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(mediaPart, file); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	uploadURL := "https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart"
	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/related; boundary=%s", writer.Boundary()))

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		errBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("upload error (%d): %s", res.StatusCode, string(errBytes))
	}

	var uploaded gdriveFileResponse
	if err := json.NewDecoder(res.Body).Decode(&uploaded); err != nil {
		return nil, err
	}
	return &uploaded, nil
}
