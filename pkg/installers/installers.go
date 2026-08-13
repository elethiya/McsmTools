package installers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"McsmTools/pkg/config"
	"McsmTools/pkg/mcserver"
)

type DownloadStatus struct {
	IsDownloading bool    `json:"is_downloading"`
	Progress      float64 `json:"progress"`
	Message       string  `json:"message"`
	FileName      string  `json:"file_name"`
	Error         string  `json:"error"`
}

type SoftwareLinkInfo struct {
	Name                string `json:"name"`
	Edition             string `json:"edition"`
	Website             string `json:"website"`
	APIURL              string `json:"api_url,omitempty"`
	ManifestURL         string `json:"manifest_url,omitempty"`
	BuildsURLTemplate   string `json:"builds_url_template,omitempty"`
	DownloadURLTemplate string `json:"download_url_template,omitempty"`
	PaperAPIURL         string `json:"paper_api_url,omitempty"`
	GeyserPluginURL     string `json:"geyser_plugin_url,omitempty"`
}

type LinksConfig struct {
	SoftwareLinks map[string]SoftwareLinkInfo `json:"software_links"`
}

func GetSoftwareLinks() map[string]SoftwareLinkInfo {
	linksPath := "links.json"
	if data, err := os.ReadFile(linksPath); err == nil {
		var cfg LinksConfig
		if err := json.Unmarshal(data, &cfg); err == nil && len(cfg.SoftwareLinks) > 0 {
			return cfg.SoftwareLinks
		}
	}
	return map[string]SoftwareLinkInfo{
		"paper": {
			Name:                "PaperMC",
			Edition:             "java",
			Website:             "https://papermc.io",
			APIURL:              "https://api.papermc.io/v2/projects/paper",
			DownloadURLTemplate: "https://api.papermc.io/v2/projects/paper/versions/{version}/builds/{build}/downloads/paper-{version}-{build}.jar",
		},
		"purpur": {
			Name:                "Purpur",
			Edition:             "java",
			Website:             "https://purpurmc.org",
			DownloadURLTemplate: "https://api.purpurmc.org/v2/purpur/{version}/latest/download",
		},
		"vanilla": {
			Name:        "Official Vanilla (Mojang)",
			Edition:     "java",
			Website:     "https://www.minecraft.net",
			ManifestURL: "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json",
		},
		"fabric": {
			Name:                "Fabric",
			Edition:             "java",
			Website:             "https://fabricmc.net",
			DownloadURLTemplate: "https://meta.fabricmc.net/v2/versions/loader/{version}/0.15.7/1.0.1/server/jar",
		},
		"geyser": {
			Name:            "Paper + GeyserMC (Cross-Play)",
			Edition:         "bedrock_crossplay",
			Website:         "https://geysermc.org",
			GeyserPluginURL: "https://download.geysermc.org/v2/projects/geyser/versions/latest/builds/latest/downloads/spigot",
		},
	}
}

type SoftwareOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type SoftwaresConfig map[string][]SoftwareOption

func GetSoftwareOptions() SoftwaresConfig {
	if data, err := os.ReadFile("softwares.json"); err == nil {
		var cfg SoftwaresConfig
		if err := json.Unmarshal(data, &cfg); err == nil {
			return cfg
		}
	}
	return SoftwaresConfig{}
}

func GetPresetVersions() map[string][]string {
	if data, err := os.ReadFile("versions.json"); err == nil {
		var versions map[string][]string
		if err := json.Unmarshal(data, &versions); err == nil {
			return versions
		}
	}
	return map[string][]string{}
}

var currentStatus = &DownloadStatus{}

func GetStatus() DownloadStatus {
	return *currentStatus
}

type PaperV3ProjectResponse struct {
	Versions map[string][]string `json:"versions"`
}

type PaperV3BuildDownload struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type PaperV3BuildItem struct {
	ID        int                             `json:"id"`
	Downloads map[string]PaperV3BuildDownload `json:"downloads"`
}

func FetchPaperVersions() ([]string, error) {
	req, err := http.NewRequest("GET", "https://fill.papermc.io/v3/projects/paper", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "McsmTools/1.0 (admin@elethiya.com)")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data PaperV3ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var allVersions []string
	for _, verList := range data.Versions {
		allVersions = append(allVersions, verList...)
	}

	if len(allVersions) > 0 {
		return allVersions, nil
	}

	return []string{"1.20.4", "1.20.2", "1.20.1", "1.19.4", "1.18.2", "1.16.5"}, nil
}

func DownloadPaperJar(version string) error {
	cfg := config.GlobalConfig
	destDir, err := filepath.Abs(cfg.ServerDir)
	if err != nil {
		return err
	}
	return DownloadPaperJarIntoDir(destDir, version)
}

func DownloadPaperJarIntoDir(destDir, version string) error {
	version = strings.TrimSpace(version)
	if parts := strings.Fields(version); len(parts) > 0 {
		version = parts[len(parts)-1]
	}
	if version == "" {
		version = "1.20.4"
	}

	if currentStatus.IsDownloading {
		return fmt.Errorf("a download is already in progress")
	}

	currentStatus = &DownloadStatus{
		IsDownloading: true,
		Progress:      0,
		Message:       fmt.Sprintf("Fetching Paper V3 build details for version %s...", version),
	}

	go func() {
		defer func() { currentStatus.IsDownloading = false }()

		buildsUrl := fmt.Sprintf("https://fill.papermc.io/v3/projects/paper/versions/%s/builds", version)
		req, err := http.NewRequest("GET", buildsUrl, nil)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Failed to create request: %v", err)
			return
		}
		req.Header.Set("User-Agent", "McsmTools/1.0 (admin@elethiya.com)")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Failed to get Paper builds: %v", err)
			return
		}
		defer resp.Body.Close()

		var builds []PaperV3BuildItem
		if err := json.NewDecoder(resp.Body).Decode(&builds); err != nil || len(builds) == 0 {
			currentStatus.Error = fmt.Sprintf("No Paper builds found for version %s", version)
			return
		}

		latestBuild := builds[len(builds)-1]
		dl, exists := latestBuild.Downloads["server:default"]
		if !exists {
			dl, exists = latestBuild.Downloads["server"]
		}

		var downloadUrl string
		var fileName string

		if exists && dl.URL != "" {
			downloadUrl = dl.URL
			fileName = dl.Name
			if fileName == "" {
				fileName = fmt.Sprintf("paper-%s-%d.jar", version, latestBuild.ID)
			}
			if !strings.HasPrefix(downloadUrl, "http://") && !strings.HasPrefix(downloadUrl, "https://") {
				downloadUrl = "https://fill.papermc.io/v3" + downloadUrl
			}
		} else {
			fileName = fmt.Sprintf("paper-%s-%d.jar", version, latestBuild.ID)
			downloadUrl = fmt.Sprintf("https://fill.papermc.io/v3/projects/paper/versions/%s/builds/%d/downloads/%s", version, latestBuild.ID, fileName)
		}

		currentStatus.Message = fmt.Sprintf("Downloading %s...", fileName)
		currentStatus.FileName = fileName

		err = downloadFileToDir(downloadUrl, fileName, destDir)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Download failed: %v", err)
			return
		}

		currentStatus.Progress = 100
		currentStatus.Message = fmt.Sprintf("Successfully downloaded %s and set as default server jar!", fileName)
		mcserver.GetManager().BroadcastLog(fmt.Sprintf("[McsmTools] Downloaded server jar: %s", fileName))
	}()

	return nil
}

func DownloadServerSoftwareIntoDir(destDir, software, version, customURL string) error {
	if software == "" && customURL == "" {
		return nil
	}
	switch software {
	case "paper":
		return DownloadPaperJarIntoDir(destDir, version)
	case "purpur":
		return DownloadPurpurJarIntoDir(destDir, version)
	case "vanilla", "official":
		return DownloadVanillaJarIntoDir(destDir, version)
	case "fabric":
		return DownloadFabricJarIntoDir(destDir, version)
	case "geyser":
		return DownloadGeyserPaperJarIntoDir(destDir, version)
	case "custom":
		if customURL != "" {
			return downloadFileToDirBackground(customURL, "server.jar", destDir, "Custom Jar")
		}
		return nil
	default:
		if version != "" {
			return DownloadPaperJarIntoDir(destDir, version)
		} else if customURL != "" {
			return downloadFileToDirBackground(customURL, "server.jar", destDir, "Server Jar")
		}
	}
	return nil
}

func cleanVersion(v string, fallback string) string {
	v = strings.TrimSpace(v)
	if parts := strings.Fields(v); len(parts) > 0 {
		v = parts[len(parts)-1]
	}
	if v == "" {
		return fallback
	}
	return v
}

func DownloadPurpurJarIntoDir(destDir, version string) error {
	version = cleanVersion(version, "1.20.4")
	url := fmt.Sprintf("https://api.purpurmc.org/v2/purpur/%s/latest/download", version)
	return downloadFileToDirBackground(url, "server.jar", destDir, fmt.Sprintf("Purpur %s", version))
}

type MojangManifest struct {
	Versions []struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"versions"`
}

type MojangVersionPkg struct {
	Downloads struct {
		Server struct {
			URL string `json:"url"`
		} `json:"server"`
	} `json:"downloads"`
}

func DownloadVanillaJarIntoDir(destDir, version string) error {
	version = cleanVersion(version, "1.20.4")
	if currentStatus.IsDownloading {
		return fmt.Errorf("a download is already in progress")
	}

	currentStatus = &DownloadStatus{
		IsDownloading: true,
		Progress:      0,
		Message:       fmt.Sprintf("Fetching Official Mojang manifest for version %s...", version),
	}

	go func() {
		defer func() { currentStatus.IsDownloading = false }()

		req, err := http.NewRequest("GET", "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json", nil)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Failed to create request: %v", err)
			return
		}
		req.Header.Set("User-Agent", "McsmTools/1.0 (admin@elethiya.com)")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Failed to fetch version manifest: %v", err)
			return
		}
		defer resp.Body.Close()

		var manifest MojangManifest
		if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
			currentStatus.Error = fmt.Sprintf("Failed to decode manifest: %v", err)
			return
		}

		var pkgURL string
		for _, v := range manifest.Versions {
			if v.ID == version {
				pkgURL = v.URL
				break
			}
		}

		if pkgURL == "" {
			currentStatus.Error = fmt.Sprintf("Vanilla version %s not found in Mojang manifest", version)
			return
		}

		reqPkg, err := http.NewRequest("GET", pkgURL, nil)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Failed to create package request: %v", err)
			return
		}
		reqPkg.Header.Set("User-Agent", "McsmTools/1.0 (admin@elethiya.com)")

		respPkg, err := client.Do(reqPkg)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Failed to fetch version package: %v", err)
			return
		}
		defer respPkg.Body.Close()

		var vPkg MojangVersionPkg
		if err := json.NewDecoder(respPkg.Body).Decode(&vPkg); err != nil || vPkg.Downloads.Server.URL == "" {
			currentStatus.Error = "Failed to parse server download link from Mojang"
			return
		}

		currentStatus.Message = fmt.Sprintf("Downloading Official Minecraft %s server.jar...", version)
		currentStatus.FileName = "server.jar"

		err = downloadFileToDir(vPkg.Downloads.Server.URL, "server.jar", destDir)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Download failed: %v", err)
			return
		}

		currentStatus.Progress = 100
		currentStatus.Message = fmt.Sprintf("Successfully downloaded Official Minecraft %s server.jar!", version)
		mcserver.GetManager().BroadcastLog(fmt.Sprintf("[McsmTools] Downloaded Official Minecraft %s server.jar", version))
	}()

	return nil
}

func DownloadFabricJarIntoDir(destDir, version string) error {
	version = cleanVersion(version, "1.20.4")
	url := fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s/0.15.7/1.0.1/server/jar", version)
	return downloadFileToDirBackground(url, "server.jar", destDir, fmt.Sprintf("Fabric %s", version))
}

func DownloadGeyserPaperJarIntoDir(destDir, version string) error {
	version = cleanVersion(version, "1.20.4")
	err := DownloadPaperJarIntoDir(destDir, version)
	if err != nil {
		return err
	}

	go func() {
		time.Sleep(5 * time.Second)
		pluginsDir := filepath.Join(destDir, "plugins")
		_ = os.MkdirAll(pluginsDir, 0755)
		geyserURL := "https://download.geysermc.org/v2/projects/geyser/versions/latest/builds/latest/downloads/spigot"
		_ = downloadFileToDir(geyserURL, "Geyser-Spigot.jar", pluginsDir)
		mcserver.GetManager().BroadcastLog("[McsmTools] Installed Geyser-Spigot plugin for Bedrock cross-play!")
	}()

	return nil
}

func downloadFileToDirBackground(url, fileName, destDir, title string) error {
	if currentStatus.IsDownloading {
		return fmt.Errorf("a download is already in progress")
	}

	currentStatus = &DownloadStatus{
		IsDownloading: true,
		Progress:      0,
		Message:       fmt.Sprintf("Downloading %s...", title),
		FileName:      fileName,
	}

	go func() {
		defer func() { currentStatus.IsDownloading = false }()

		err := downloadFileToDir(url, fileName, destDir)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Download failed: %v", err)
			return
		}

		currentStatus.Progress = 100
		currentStatus.Message = fmt.Sprintf("Successfully downloaded %s!", title)
		mcserver.GetManager().BroadcastLog(fmt.Sprintf("[McsmTools] Downloaded %s into server directory", title))
	}()

	return nil
}

func DownloadFromURL(url string, targetName string) error {
	cfg := config.GlobalConfig
	destDir, err := filepath.Abs(cfg.ServerDir)
	if err != nil {
		return err
	}
	return DownloadFromURLToDir(url, targetName, destDir)
}

func DownloadFromURLToDir(url string, targetName string, destDir string) error {
	if currentStatus.IsDownloading {
		return fmt.Errorf("a download is already in progress")
	}

	if targetName == "" {
		targetName = "server.jar"
	}

	return downloadFileToDirBackground(url, targetName, destDir, targetName)
}

func downloadFileToDir(url string, fileName string, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	destPath := filepath.Join(destDir, fileName)
	jarPath := filepath.Join(destDir, "server.jar")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "McsmTools/1.0 (Mozilla/5.0)")

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	totalSize := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := out.Write(buf[0:nr])
			if nw > 0 {
				downloaded += int64(nw)
				if totalSize > 0 {
					currentStatus.Progress = float64(downloaded) / float64(totalSize) * 100
				}
			}
			if ew != nil {
				return ew
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			return er
		}
	}

	if fileName != "server.jar" {
		copyFile(destPath, jarPath)
	}

	return nil
}

func copyFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer out.Close()

	_, _ = io.Copy(out, in)
}
