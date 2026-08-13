package installers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"void-panel/pkg/config"
	"void-panel/pkg/mcserver"
)

type DownloadStatus struct {
	IsDownloading bool    `json:"is_downloading"`
	Progress      float64 `json:"progress"`
	Message       string  `json:"message"`
	FileName      string  `json:"file_name"`
	Error         string  `json:"error"`
}

var currentStatus = &DownloadStatus{}

func GetStatus() DownloadStatus {
	return *currentStatus
}

type PaperVersionResponse struct {
	Versions []string `json:"versions"`
}

type PaperBuildsResponse struct {
	Builds []int `json:"builds"`
}

func FetchPaperVersions() ([]string, error) {
	resp, err := http.Get("https://api.papermc.io/v2/projects/paper")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data PaperVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Versions, nil
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
	if currentStatus.IsDownloading {
		return fmt.Errorf("a download is already in progress")
	}

	currentStatus = &DownloadStatus{
		IsDownloading: true,
		Progress:      0,
		Message:       fmt.Sprintf("Fetching Paper build details for version %s...", version),
	}

	go func() {
		defer func() { currentStatus.IsDownloading = false }()

		buildsUrl := fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s", version)
		resp, err := http.Get(buildsUrl)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Failed to get Paper builds: %v", err)
			return
		}
		defer resp.Body.Close()

		var buildsData PaperBuildsResponse
		if err := json.NewDecoder(resp.Body).Decode(&buildsData); err != nil || len(buildsData.Builds) == 0 {
			currentStatus.Error = "No builds found for this version"
			return
		}

		latestBuild := buildsData.Builds[len(buildsData.Builds)-1]
		fileName := fmt.Sprintf("paper-%s-%d.jar", version, latestBuild)
		downloadUrl := fmt.Sprintf("https://api.papermc.io/v2/projects/paper/versions/%s/builds/%d/downloads/%s", version, latestBuild, fileName)

		currentStatus.Message = fmt.Sprintf("Downloading %s...", fileName)
		currentStatus.FileName = fileName

		err = downloadFileToDir(downloadUrl, fileName, destDir)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Download failed: %v", err)
			return
		}

		currentStatus.Progress = 100
		currentStatus.Message = fmt.Sprintf("Successfully downloaded %s and set as default server jar!", fileName)
		mcserver.GetManager().BroadcastLog(fmt.Sprintf("[VoidPanel] Downloaded server jar: %s", fileName))
	}()

	return nil
}

func DownloadFromURL(url string, targetName string) error {
	cfg := config.GlobalConfig
	destDir, err := filepath.Abs(cfg.ServerDir)
	if err != nil {
		return err
	}

	if currentStatus.IsDownloading {
		return fmt.Errorf("a download is already in progress")
	}

	if targetName == "" {
		targetName = "server.jar"
	}

	currentStatus = &DownloadStatus{
		IsDownloading: true,
		Progress:      0,
		Message:       fmt.Sprintf("Starting download from %s...", url),
		FileName:      targetName,
	}

	go func() {
		defer func() { currentStatus.IsDownloading = false }()

		err := downloadFileToDir(url, targetName, destDir)
		if err != nil {
			currentStatus.Error = fmt.Sprintf("Download failed: %v", err)
			return
		}

		currentStatus.Progress = 100
		currentStatus.Message = fmt.Sprintf("Successfully downloaded %s!", targetName)
		mcserver.GetManager().BroadcastLog(fmt.Sprintf("[VoidPanel] Downloaded server jar from URL: %s", targetName))
	}()

	return nil
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
