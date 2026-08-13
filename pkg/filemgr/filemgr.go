package filemgr

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"void-panel/pkg/config"
)

type FileItem struct {
	Name       string    `json:"name"`
	RelPath    string    `json:"rel_path"`
	IsDir      bool      `json:"is_dir"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"mod_time"`
	Extension  string    `json:"extension"`
	IsEditable bool      `json:"is_editable"`
}

type FileContent struct {
	Name      string    `json:"name"`
	RelPath   string    `json:"rel_path"`
	Content   string    `json:"content"`
	Language  string    `json:"language"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	ReadOnly  bool      `json:"read_only"`
}

func getRootDir() (string, error) {
	cfg := config.GlobalConfig
	if cfg == nil {
		return "", fmt.Errorf("config not loaded")
	}
	abs, err := filepath.Abs(cfg.ServerDir)
	if err != nil {
		return "", err
	}
	_ = os.MkdirAll(abs, 0755)
	return abs, nil
}

func resolveSafePath(rel string) (string, string, error) {
	root, err := getRootDir()
	if err != nil {
		return "", "", err
	}

	cleanRel := filepath.Clean("/" + rel)
	cleanRel = strings.TrimPrefix(cleanRel, "/")

	targetPath := filepath.Join(root, cleanRel)
	
	// Security check to prevent path traversal outside server directory
	if !strings.HasPrefix(targetPath, root) {
		return "", "", fmt.Errorf("access denied: path outside server root")
	}

	return root, targetPath, nil
}

func ListDirectory(relPath string) ([]FileItem, string, error) {
	_, absPath, err := resolveSafePath(relPath)
	if err != nil {
		return nil, "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, "", fmt.Errorf("directory not found: %v", err)
	}

	if !info.IsDir() {
		return nil, "", fmt.Errorf("path is not a directory")
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read directory: %v", err)
	}

	items := make([]FileItem, 0, len(entries))
	root, _ := getRootDir()
	cleanRel, _ := filepath.Rel(root, absPath)
	if cleanRel == "." {
		cleanRel = ""
	}

	for _, entry := range entries {
		eInfo, err := entry.Info()
		if err != nil {
			continue
		}

		entryRel := filepath.Join(cleanRel, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		isEditable := !entry.IsDir() && isTextExtension(ext)

		items = append(items, FileItem{
			Name:       entry.Name(),
			RelPath:    entryRel,
			IsDir:      entry.IsDir(),
			Size:       eInfo.Size(),
			ModTime:    eInfo.ModTime(),
			Extension:  ext,
			IsEditable: isEditable,
		})
	}

	return items, cleanRel, nil
}

func ReadFileContent(relPath string) (*FileContent, error) {
	_, absPath, err := resolveSafePath(relPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %v", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("cannot open directory as text file")
	}

	// 10MB limit for web editor
	if info.Size() > 10*1024*1024 {
		return nil, fmt.Errorf("file size too large (>10MB) to open in browser editor")
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	ext := strings.ToLower(filepath.Ext(info.Name()))
	lang := detectLanguage(info.Name(), ext)

	root, _ := getRootDir()
	cleanRel, _ := filepath.Rel(root, absPath)

	return &FileContent{
		Name:     info.Name(),
		RelPath:  cleanRel,
		Content:  string(data),
		Language: lang,
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		ReadOnly: false,
	}, nil
}

func SaveFileContent(relPath string, content string) error {
	_, absPath, err := resolveSafePath(relPath)
	if err != nil {
		return err
	}

	parent := filepath.Dir(absPath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %v", err)
	}

	return os.WriteFile(absPath, []byte(content), 0644)
}

func CreateFile(relPath string) error {
	_, absPath, err := resolveSafePath(relPath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(absPath); err == nil {
		return fmt.Errorf("file already exists")
	}

	parent := filepath.Dir(absPath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %v", err)
	}

	f, err := os.Create(absPath)
	if err != nil {
		return err
	}
	return f.Close()
}

func CreateFolder(relPath string) error {
	_, absPath, err := resolveSafePath(relPath)
	if err != nil {
		return err
	}

	return os.MkdirAll(absPath, 0755)
}

func RenameItem(oldRelPath, newRelPath string) error {
	_, oldAbs, err := resolveSafePath(oldRelPath)
	if err != nil {
		return err
	}

	_, newAbs, err := resolveSafePath(newRelPath)
	if err != nil {
		return err
	}

	parent := filepath.Dir(newAbs)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}

	return os.Rename(oldAbs, newAbs)
}

func DeleteItem(relPath string) error {
	root, absPath, err := resolveSafePath(relPath)
	if err != nil {
		return err
	}

	if absPath == root {
		return fmt.Errorf("cannot delete root server directory")
	}

	return os.RemoveAll(absPath)
}

func ZipItem(relPath string) (string, error) {
	root, absPath, err := resolveSafePath(relPath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", err
	}

	zipName := info.Name() + ".zip"
	zipAbsPath := filepath.Join(filepath.Dir(absPath), zipName)

	zipFile, err := os.Create(zipAbsPath)
	if err != nil {
		return "", err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relInZip, err := filepath.Rel(filepath.Dir(absPath), path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			_, err = archive.Create(relInZip + "/")
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relInZip
		header.Method = zip.Deflate

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	if err != nil {
		return "", err
	}

	relZip, _ := filepath.Rel(root, zipAbsPath)
	return relZip, nil
}

func UnzipItem(relPath string) error {
	root, absPath, err := resolveSafePath(relPath)
	if err != nil {
		return err
	}

	reader, err := zip.OpenReader(absPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %v", err)
	}
	defer reader.Close()

	targetDir := filepath.Dir(absPath)

	for _, f := range reader.File {
		fPath := filepath.Join(targetDir, f.Name)

		if !strings.HasPrefix(fPath, filepath.Clean(targetDir)+string(os.PathSeparator)) && fPath != targetDir {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(fPath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fPath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	_ = root
	return nil
}

func isTextExtension(ext string) bool {
	switch ext {
	case ".yml", ".yaml", ".json", ".properties", ".txt", ".log", ".sk",
		".py", ".sh", ".bash", ".js", ".ts", ".html", ".css", ".xml",
		".conf", ".cfg", ".toml", ".md", ".java", ".go", ".c", ".cpp",
		".ini", ".env", ".mcmeta", ".lock":
		return true
	default:
		return false
	}
}

func detectLanguage(filename, ext string) string {
	if strings.HasPrefix(filename, "server.properties") {
		return "properties"
	}
	switch ext {
	case ".yml", ".yaml":
		return "yaml"
	case ".json", ".mcmeta":
		return "json"
	case ".properties", ".conf", ".cfg", ".ini", ".env":
		return "properties"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".sh", ".bash":
		return "shell"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".xml":
		return "xml"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".java":
		return "java"
	case ".go":
		return "go"
	case ".sk":
		return "skript"
	default:
		return "plaintext"
	}
}
