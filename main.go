package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"void-panel/pkg/auth"
	"void-panel/pkg/config"
	"void-panel/pkg/filemgr"
	"void-panel/pkg/installers"
	"void-panel/pkg/logger"
	"void-panel/pkg/mcserver"
	"void-panel/pkg/metrics"
	"void-panel/pkg/players"
	"void-panel/pkg/plugins"
	"void-panel/pkg/servermgr"
	"void-panel/pkg/websocket"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func httpLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		duration := time.Since(start)

		if !strings.HasPrefix(r.URL.Path, "/static/") && r.URL.Path != "/ws" {
			logger.HTTPLog("%s %s %d (%v)", r.Method, r.URL.Path, lrw.statusCode, duration)
		}
	})
}

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load config: %v", err)
		os.Exit(1)
	}

	if err := servermgr.LoadServers(); err != nil {
		logger.Warn("Failed to load multi-server instances: %v", err)
	}

	logger.Info("====================================================")
	logger.Info("  VOID PANEL - Minecraft Server Dashboard & Control  ")
	logger.Info("  Port: %d", cfg.Port)
	logger.Info("  Default Auth: %s / %s", cfg.Username, cfg.Password)
	logger.Info("  Active Server Dir: %s", cfg.ServerDir)
	logger.Info("====================================================")

	mux := http.NewServeMux()

	// Static Assets
	fs := http.FileServer(http.Dir("./web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// HTML Pages
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if auth.IsAuthenticated(r) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		http.ServeFile(w, r, "./web/templates/login.html")
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if !auth.IsAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		http.ServeFile(w, r, "./web/templates/index.html")
	})

	// Auth Endpoints
	mux.HandleFunc("/api/auth/login", handleLogin)
	mux.HandleFunc("/api/auth/logout", handleLogout)
	mux.HandleFunc("/api/auth/status", handleAuthStatus)

	// Multi-Server Instances APIs
	mux.HandleFunc("/api/servers/list", auth.RequireAuth(handleServerList))
	mux.HandleFunc("/api/servers/create", auth.RequireAuth(handleServerCreate))
	mux.HandleFunc("/api/servers/switch", auth.RequireAuth(handleServerSwitch))
	mux.HandleFunc("/api/servers/delete", auth.RequireAuth(handleServerDelete))

	// Protected API Routes
	mux.HandleFunc("/api/server/start", auth.RequireAuth(handleServerStart))
	mux.HandleFunc("/api/server/stop", auth.RequireAuth(handleServerStop))
	mux.HandleFunc("/api/server/restart", auth.RequireAuth(handleServerRestart))
	mux.HandleFunc("/api/server/kill", auth.RequireAuth(handleServerKill))
	mux.HandleFunc("/api/server/command", auth.RequireAuth(handleServerCommand))
	mux.HandleFunc("/api/server/metrics", auth.RequireAuth(handleServerMetrics))

	// File & Code Management APIs
	mux.HandleFunc("/api/files/list", auth.RequireAuth(handleFileList))
	mux.HandleFunc("/api/files/read", auth.RequireAuth(handleFileRead))
	mux.HandleFunc("/api/files/save", auth.RequireAuth(handleFileSave))
	mux.HandleFunc("/api/files/create-file", auth.RequireAuth(handleFileCreate))
	mux.HandleFunc("/api/files/create-folder", auth.RequireAuth(handleFolderCreate))
	mux.HandleFunc("/api/files/rename", auth.RequireAuth(handleFileRename))
	mux.HandleFunc("/api/files/delete", auth.RequireAuth(handleFileDelete))
	mux.HandleFunc("/api/files/upload", auth.RequireAuth(handleFileUpload))
	mux.HandleFunc("/api/files/download", auth.RequireAuth(handleFileDownload))
	mux.HandleFunc("/api/files/zip", auth.RequireAuth(handleFileZip))
	mux.HandleFunc("/api/files/unzip", auth.RequireAuth(handleFileUnzip))

	// Plugin Store APIs
	mux.HandleFunc("/api/plugins/featured", auth.RequireAuth(handlePluginsFeatured))
	mux.HandleFunc("/api/plugins/search", auth.RequireAuth(handlePluginsSearch))
	mux.HandleFunc("/api/plugins/install", auth.RequireAuth(handlePluginsInstall))

	// Config & Server Properties APIs
	mux.HandleFunc("/api/config/properties", auth.RequireAuth(handleProperties))
	mux.HandleFunc("/api/settings", auth.RequireAuth(handleSettings))

	// Players API
	mux.HandleFunc("/api/players/list", auth.RequireAuth(handlePlayersList))
	mux.HandleFunc("/api/players/action", auth.RequireAuth(handlePlayerAction))

	// Installer API
	mux.HandleFunc("/api/installer/status", auth.RequireAuth(handleInstallerStatus))
	mux.HandleFunc("/api/installer/paper-versions", auth.RequireAuth(handlePaperVersions))
	mux.HandleFunc("/api/installer/download-paper", auth.RequireAuth(handleDownloadPaper))
	mux.HandleFunc("/api/installer/download-url", auth.RequireAuth(handleDownloadURL))

	// WebSocket Endpoint
	mux.HandleFunc("/ws", websocket.ServeWS)

	handlerWithLogging := httpLoggingMiddleware(mux)

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("Server running on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, handlerWithLogging); err != nil {
		logger.Error("Server error: %v", err)
	}
}

// JSON Helper
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Handlers Implementation

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if auth.Authenticate(req.Username, req.Password) {
		token := auth.CreateSession(w)
		logger.AuthLog("User '%s' logged in successfully", req.Username)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"token":   token,
		})
	} else {
		logger.AuthLog("Failed login attempt for username '%s'", req.Username)
		writeError(w, http.StatusUnauthorized, "Invalid username or password")
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	auth.Logout(w, r)
	logger.AuthLog("User logged out")
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"authenticated": auth.IsAuthenticated(r),
	})
}

// Multi-Server Instances Handlers
func handleServerList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active_id": servermgr.GetActiveID(),
		"servers":   servermgr.ListServers(),
	})
}

func handleServerCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		PaperVersion string `json:"version"`
		Port         int    `json:"port"`
		MemoryMin    string `json:"memory_min"`
		MemoryMax    string `json:"memory_max"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	srv, err := servermgr.CreateServer(req.Name, req.PaperVersion, req.Port, req.MemoryMin, req.MemoryMax)
	if err != nil {
		logger.Error("Failed to create server instance '%s': %v", req.Name, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Success("Created server instance '%s' (ID: %s, Port: %d)", req.Name, srv.ID, srv.Port)
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "server": srv})
}

func handleServerSwitch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, "Server ID required")
		return
	}
	if err := servermgr.SwitchServer(req.ID); err != nil {
		logger.Error("Failed to switch server to ID '%s': %v", req.ID, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("Switched active server instance to ID '%s'", req.ID)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleServerDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID          string `json:"id"`
		DeleteFiles bool   `json:"delete_files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		writeError(w, http.StatusBadRequest, "Server ID required")
		return
	}
	if err := servermgr.DeleteServer(req.ID, req.DeleteFiles); err != nil {
		logger.Error("Failed to delete server ID '%s': %v", req.ID, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Warn("Deleted server instance ID '%s' (DeleteFiles: %v)", req.ID, req.DeleteFiles)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleServerStart(w http.ResponseWriter, r *http.Request) {
	if err := mcserver.GetManager().Start(); err != nil {
		logger.Error("Failed to start server: %v", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("Server start command initiated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "Server start initiated"})
}

func handleServerStop(w http.ResponseWriter, r *http.Request) {
	if err := mcserver.GetManager().Stop(); err != nil {
		logger.Error("Failed to stop server: %v", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("Server stop command initiated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "Server stop initiated"})
}

func handleServerRestart(w http.ResponseWriter, r *http.Request) {
	if err := mcserver.GetManager().Restart(); err != nil {
		logger.Error("Failed to restart server: %v", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("Server restart command initiated")
	writeJSON(w, http.StatusOK, map[string]string{"message": "Server restart initiated"})
}

func handleServerKill(w http.ResponseWriter, r *http.Request) {
	if err := mcserver.GetManager().Kill(); err != nil {
		logger.Error("Failed to kill server process: %v", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Warn("Server process killed by user")
	writeJSON(w, http.StatusOK, map[string]string{"message": "Server process killed"})
}

func handleServerCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := mcserver.GetManager().SendCommand(req.Command); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.MCServer("Console input: %s", req.Command)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleServerMetrics(w http.ResponseWriter, r *http.Request) {
	m, err := metrics.CollectMetrics()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// File & Code Management Handlers
func handleFileList(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	items, cleanRel, err := filemgr.ListDirectory(relPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"current_path": cleanRel,
		"items":        items,
	})
}

func handleFileRead(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	fileContent, err := filemgr.ReadFileContent(relPath)
	if err != nil {
		logger.Error("Failed to read file '%s': %v", relPath, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.FileLog("Opened file '%s' for editing", relPath)
	writeJSON(w, http.StatusOK, fileContent)
}

func handleFileSave(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if err := filemgr.SaveFileContent(req.Path, req.Content); err != nil {
		logger.Error("Failed to save file '%s': %v", req.Path, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.FileLog("Saved changes to file '%s'", req.Path)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleFileCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "File path required")
		return
	}
	if err := filemgr.CreateFile(req.Path); err != nil {
		logger.Error("Failed to create file '%s': %v", req.Path, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.FileLog("Created new file '%s'", req.Path)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleFolderCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "Folder path required")
		return
	}
	if err := filemgr.CreateFolder(req.Path); err != nil {
		logger.Error("Failed to create folder '%s': %v", req.Path, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.FileLog("Created new folder '%s'", req.Path)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleFileRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OldPath == "" || req.NewPath == "" {
		writeError(w, http.StatusBadRequest, "Old and new paths required")
		return
	}
	if err := filemgr.RenameItem(req.OldPath, req.NewPath); err != nil {
		logger.Error("Failed to rename '%s' to '%s': %v", req.OldPath, req.NewPath, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.FileLog("Renamed '%s' to '%s'", req.OldPath, req.NewPath)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleFileDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "Path required")
		return
	}
	if err := filemgr.DeleteItem(req.Path); err != nil {
		logger.Error("Failed to delete '%s': %v", req.Path, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.FileLog("Deleted '%s'", req.Path)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(100 << 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Upload too large or invalid form")
		return
	}

	targetDir := r.FormValue("target_dir")
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	destRel := filepath.Join(targetDir, header.Filename)
	destAbs := filepath.Join(config.GlobalConfig.ServerDir, destRel)

	if err := os.MkdirAll(filepath.Dir(destAbs), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create upload dir")
		return
	}

	out, err := os.Create(destAbs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to write file")
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save file contents")
		return
	}

	logger.FileLog("Uploaded file '%s' to '%s'", header.Filename, targetDir)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleFileDownload(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	absPath := filepath.Join(config.GlobalConfig.ServerDir, relPath)

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusBadRequest, "File not found or is a directory")
		return
	}

	logger.FileLog("Downloaded file '%s'", relPath)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", info.Name()))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, absPath)
}

func handleFileZip(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "Path required")
		return
	}
	zipRel, err := filemgr.ZipItem(req.Path)
	if err != nil {
		logger.Error("Failed to zip '%s': %v", req.Path, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.FileLog("Zipped '%s' to '%s'", req.Path, zipRel)
	writeJSON(w, http.StatusOK, map[string]string{"zip_path": zipRel})
}

func handleFileUnzip(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "Path required")
		return
	}
	if err := filemgr.UnzipItem(req.Path); err != nil {
		logger.Error("Failed to unzip '%s': %v", req.Path, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.FileLog("Unzipped archive '%s'", req.Path)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// Plugin Store Handlers
func handlePluginsFeatured(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, plugins.GetFeaturedPlugins())
}

func handlePluginsSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	results, err := plugins.SearchModrinthPlugins(query)
	if err != nil {
		logger.Error("Plugin search error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func handlePluginsInstall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeError(w, http.StatusBadRequest, "Download URL required")
		return
	}
	if err := plugins.InstallPluginJar(req.URL, req.Name); err != nil {
		logger.Error("Failed to install plugin '%s': %v", req.Name, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Success("Installed plugin '%s' from %s", req.Name, req.URL)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// Config & Properties
func handleProperties(w http.ResponseWriter, r *http.Request) {
	propsPath := filepath.Join(config.GlobalConfig.ServerDir, "server.properties")

	if r.Method == http.MethodGet {
		data, err := os.ReadFile(propsPath)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{})
			return
		}
		propsMap := parseProperties(string(data))
		writeJSON(w, http.StatusOK, propsMap)
		return
	}

	if r.Method == http.MethodPost {
		var props map[string]string
		if err := json.NewDecoder(r.Body).Decode(&props); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON payload")
			return
		}

		var lines []string
		lines = append(lines, "# Minecraft server properties")
		lines = append(lines, "# Generated by VoidPanel")
		for k, v := range props {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}

		content := strings.Join(lines, "\n")
		if err := os.WriteFile(propsPath, []byte(content), 0644); err != nil {
			logger.Error("Failed to save server.properties: %v", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		logger.Success("Updated server.properties")
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}
}

func parseProperties(raw string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, config.GlobalConfig)
		return
	}

	if r.Method == http.MethodPost {
		var newCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid configuration format")
			return
		}
		if err := config.SaveConfig(&newCfg); err != nil {
			logger.Error("Failed to save settings: %v", err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		logger.Success("Updated panel settings")
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	}
}

// Players
func handlePlayersList(w http.ResponseWriter, r *http.Request) {
	data, err := players.GetPlayerLists()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func handlePlayerAction(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
		Player string `json:"player"`
		Reason string `json:"reason,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Player == "" {
		writeError(w, http.StatusBadRequest, "Player name and action required")
		return
	}

	var err error
	switch req.Action {
	case "op":
		err = players.OpPlayer(req.Player)
	case "deop":
		err = players.DeopPlayer(req.Player)
	case "whitelist_add":
		err = players.WhitelistAdd(req.Player)
	case "whitelist_remove":
		err = players.WhitelistRemove(req.Player)
	case "kick":
		err = players.KickPlayer(req.Player, req.Reason)
	case "ban":
		err = players.BanPlayer(req.Player, req.Reason)
	case "unban":
		err = players.UnbanPlayer(req.Player)
	default:
		writeError(w, http.StatusBadRequest, "Unknown player action")
		return
	}

	if err != nil {
		logger.Error("Player action '%s' failed for '%s': %v", req.Action, req.Player, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.MCServer("Executed player action '%s' on '%s'", req.Action, req.Player)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// Installer Handlers
func handleInstallerStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, installers.GetStatus())
}

func handlePaperVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := installers.FetchPaperVersions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

func handleDownloadPaper(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Version == "" {
		writeError(w, http.StatusBadRequest, "Paper version required")
		return
	}
	if err := installers.DownloadPaperJar(req.Version); err != nil {
		logger.Error("Failed to initiate Paper %s download: %v", req.Version, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("Initiated Paper %s download", req.Version)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func handleDownloadURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeError(w, http.StatusBadRequest, "Valid URL required")
		return
	}
	if err := installers.DownloadFromURL(req.URL, req.Name); err != nil {
		logger.Error("Failed to initiate URL download: %v", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("Initiated custom jar download from %s", req.URL)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
