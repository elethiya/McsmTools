package plugins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"McsmTools/pkg/config"
)

type ModrinthSearchResult struct {
	Hits []ModrinthProject `json:"hits"`
}

type ModrinthProject struct {
	ProjectID    string   `json:"project_id"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Categories   []string `json:"categories"`
	Downloads    int      `json:"downloads"`
	IconURL      string   `json:"icon_url"`
	LatestVerURL string   `json:"latest_version_url,omitempty"`
}

type FeaturedPlugin struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	DownloadURL string `json:"download_url"`
	FileName    string `json:"file_name"`
	Icon        string `json:"icon"`
}

func GetFeaturedPlugins() []FeaturedPlugin {
	return []FeaturedPlugin{
		{
			Name:        "EssentialsX",
			Description: "Essential commands for teleportation, homes, economy, and server management.",
			Category:    "Admin / Essentials",
			DownloadURL: "https://github.com/EssentialsX/Essentials/releases/download/2.20.1/EssentialsX-2.20.1.jar",
			FileName:    "EssentialsX.jar",
			Icon:        "⚡",
		},
		{
			Name:        "Vault",
			Description: "Permissions, chat, and economy API hook for Minecraft plugins.",
			Category:    "Library / API",
			DownloadURL: "https://dev.bukkit.org/projects/vault/files/latest",
			FileName:    "Vault.jar",
			Icon:        "🔑",
		},
		{
			Name:        "LuckPerms",
			Description: "Advanced permissions plugin supporting ranks, groups, and web editor.",
			Category:    "Permissions",
			DownloadURL: "https://download.luckperms.net/1550/bukkit/loader/LuckPerms-Bukkit-5.4.140.jar",
			FileName:    "LuckPerms.jar",
			Icon:        "🛡️",
		},
		{
			Name:        "WorldEdit",
			Description: "In-game Minecraft map editor with selection, brush, and building tools.",
			Category:    "Building / Editing",
			DownloadURL: "https://mediafilez.forgecdn.net/files/4576/475/worldedit-bukkit-7.2.15.jar",
			FileName:    "WorldEdit.jar",
			Icon:        "🌍",
		},
		{
			Name:        "ViaVersion",
			Description: "Allows newer Minecraft client versions to connect to older server versions.",
			Category:    "Compatibility",
			DownloadURL: "https://hangarcdn.papermc.io/plugins/ViaVersion/ViaVersion/versions/4.10.0/PAPER/ViaVersion-4.10.0.jar",
			FileName:    "ViaVersion.jar",
			Icon:        "🌐",
		},
		{
			Name:        "Multiverse-Core",
			Description: "Create, manage, and port between multiple worlds on one server.",
			Category:    "World Management",
			DownloadURL: "https://github.com/Multiverse/Multiverse-Core/releases/download/4.3.1/Multiverse-Core-4.3.1.jar",
			FileName:    "Multiverse-Core.jar",
			Icon:        "🌌",
		},
		{
			Name:        "Chunky",
			Description: "Pre-generate world chunks to eliminate server lag during exploration.",
			Category:    "Performance",
			DownloadURL: "https://hangarcdn.papermc.io/plugins/popcorn/Chunky/versions/1.3.138/PAPER/Chunky-1.3.138.jar",
			FileName:    "Chunky.jar",
			Icon:        "🚀",
		},
	}
}

func SearchModrinthPlugins(query string) ([]ModrinthProject, error) {
	if query == "" {
		query = "plugin"
	}
	apiURL := fmt.Sprintf("https://api.modrinth.com/v2/search?query=%s&facets=[[\"project_type:plugin\"]]", url.QueryEscape(query))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "McsmTools/1.0.0 (admin@mcsmtools.local)")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("modrinth returned status: %s", resp.Status)
	}

	var res ModrinthSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Hits, nil
}

func InstallPluginJar(downloadURL, fileName string) error {
	cfg := config.GlobalConfig
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	pluginsDir := filepath.Join(cfg.ServerDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return err
	}

	if fileName == "" {
		fileName = "plugin.jar"
	}

	targetPath := filepath.Join(pluginsDir, fileName)

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "McsmTools/1.0.0 (admin@mcsmtools.local)")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download plugin, status: %s", resp.Status)
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
