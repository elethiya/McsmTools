package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	Port          int    `json:"port"`
	AuthEnabled   bool   `json:"auth_enabled"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	ServerDir     string `json:"server_dir"`
	JavaPath      string `json:"java_path"`
	ServerJar     string `json:"server_jar"`
	MemoryMin     string `json:"memory_min"`
	MemoryMax     string `json:"memory_max"`
	JavaFlags     string `json:"java_flags"`
	SessionSecret string `json:"session_secret"`
	AutoRestart   bool   `json:"auto_restart"`
}

var (
	GlobalConfig *Config
	configMutex  sync.RWMutex
	configPath   = "config.json"
)

func LoadConfig() (*Config, error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	cfg := &Config{
		Port:          8080,
		AuthEnabled:   true,
		Username:      "admin",
		Password:      "admin123",
		ServerDir:     "./mc_server",
		JavaPath:      "java",
		ServerJar:     "server.jar",
		MemoryMin:     "1024M",
		MemoryMax:     "2048M",
		JavaFlags:     "-XX:+UseG1GC",
		SessionSecret: generateRandomSecret(),
		AutoRestart:   false,
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		GlobalConfig = cfg
		saveConfigLocked(cfg)
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		GlobalConfig = cfg
		return cfg, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		GlobalConfig = cfg
		return cfg, err
	}

	// Ensure absolute server dir resolution
	absDir, err := filepath.Abs(cfg.ServerDir)
	if err == nil {
		_ = os.MkdirAll(absDir, 0755)
		cfg.ServerDir = absDir
	}

	GlobalConfig = cfg
	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	configMutex.Lock()
	defer configMutex.Unlock()
	GlobalConfig = cfg
	return saveConfigLocked(cfg)
}

func saveConfigLocked(cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func generateRandomSecret() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "void-panel-super-secret-key-change-me"
	}
	return hex.EncodeToString(bytes)
}
