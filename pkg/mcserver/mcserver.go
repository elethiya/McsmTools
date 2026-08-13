package mcserver

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"void-panel/pkg/config"
	"void-panel/pkg/logger"
)

type ServerStatus string

const (
	StatusStopped  ServerStatus = "STOPPED"
	StatusStarting ServerStatus = "STARTING"
	StatusRunning  ServerStatus = "RUNNING"
	StatusStopping ServerStatus = "STOPPING"
)

type Manager struct {
	mu           sync.RWMutex
	status       ServerStatus
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	startTime    time.Time
	logHistory   []string
	maxLogBuffer int
	subscribers  map[chan string]bool
	subMutex     sync.RWMutex
}

var Instance *Manager

func init() {
	Instance = &Manager{
		status:       StatusStopped,
		maxLogBuffer: 1000,
		logHistory:   make([]string, 0, 1000),
		subscribers:  make(map[chan string]bool),
	}
}

func GetManager() *Manager {
	return Instance
}

func (m *Manager) GetStatus() ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) SetStatus(status ServerStatus) {
	m.mu.Lock()
	m.status = status
	m.mu.Unlock()
	m.BroadcastLog(fmt.Sprintf("Server status changed to: %s", status))
}

func (m *Manager) GetPID() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Pid
	}
	return 0
}

func (m *Manager) GetUptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.status == StatusRunning || m.status == StatusStarting {
		return time.Since(m.startTime)
	}
	return 0
}

func (m *Manager) GetLogHistory() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	historyCopy := make([]string, len(m.logHistory))
	copy(historyCopy, m.logHistory)
	return historyCopy
}

func (m *Manager) SubscribeLogs() chan string {
	ch := make(chan string, 100)
	m.subMutex.Lock()
	m.subscribers[ch] = true
	m.subMutex.Unlock()
	return ch
}

func (m *Manager) UnsubscribeLogs(ch chan string) {
	m.subMutex.Lock()
	if _, ok := m.subscribers[ch]; ok {
		delete(m.subscribers, ch)
		close(ch)
	}
	m.subMutex.Unlock()
}

func (m *Manager) BroadcastLog(line string) {
	m.mu.Lock()
	if len(m.logHistory) >= m.maxLogBuffer {
		m.logHistory = m.logHistory[1:]
	}
	m.logHistory = append(m.logHistory, line)
	m.mu.Unlock()

	// Print to terminal with tag and log to date file
	logger.MCServer("%s", line)

	m.subMutex.RLock()
	defer m.subMutex.RUnlock()
	for ch := range m.subscribers {
		select {
		case ch <- line:
		default:
			// Buffer full, drop non-blocking
		}
	}
}

func (m *Manager) Start() error {
	m.mu.Lock()
	if m.status != StatusStopped {
		m.mu.Unlock()
		return fmt.Errorf("server is already %s", m.status)
	}
	m.status = StatusStarting
	m.mu.Unlock()

	cfg := config.GlobalConfig
	serverDir, err := filepath.Abs(cfg.ServerDir)
	if err != nil {
		m.SetStatus(StatusStopped)
		return fmt.Errorf("invalid server directory: %v", err)
	}

	if err := os.MkdirAll(serverDir, 0755); err != nil {
		m.SetStatus(StatusStopped)
		return fmt.Errorf("could not create server directory: %v", err)
	}

	jarPath := filepath.Join(serverDir, cfg.ServerJar)
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		m.SetStatus(StatusStopped)
		return fmt.Errorf("server jar not found at %s. Please download or upload a server jar file", jarPath)
	}

	// Auto check / accept EULA
	ensureEula(serverDir)

	args := []string{
		fmt.Sprintf("-Xms%s", cfg.MemoryMin),
		fmt.Sprintf("-Xmx%s", cfg.MemoryMax),
	}
	if cfg.JavaFlags != "" {
		flagParts := strings.Fields(cfg.JavaFlags)
		args = append(args, flagParts...)
	}
	args = append(args, "-jar", cfg.ServerJar, "nogui")

	m.BroadcastLog(fmt.Sprintf("Launching command: %s %s", cfg.JavaPath, strings.Join(args, " ")))
	m.BroadcastLog(fmt.Sprintf("Working Directory: %s", serverDir))

	cmd := exec.Command(cfg.JavaPath, args...)
	cmd.Dir = serverDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		m.SetStatus(StatusStopped)
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}
	m.stdin = stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.SetStatus(StatusStopped)
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.SetStatus(StatusStopped)
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		m.SetStatus(StatusStopped)
		return fmt.Errorf("failed to start minecraft process: %v", err)
	}

	m.mu.Lock()
	m.cmd = cmd
	m.startTime = time.Now()
	m.status = StatusRunning
	m.mu.Unlock()

	m.BroadcastLog(fmt.Sprintf("Process started with PID: %d", cmd.Process.Pid))

	go m.scanPipe(stdout, "STDOUT")
	go m.scanPipe(stderr, "STDERR")

	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		m.status = StatusStopped
		m.cmd = nil
		m.stdin = nil
		m.mu.Unlock()

		if err != nil {
			m.BroadcastLog(fmt.Sprintf("Server process exited with code/error: %v", err))
		} else {
			m.BroadcastLog("Server process exited cleanly.")
		}

		if config.GlobalConfig != nil && config.GlobalConfig.AutoRestart {
			m.BroadcastLog("Auto-restart enabled. Restarting in 5 seconds...")
			time.Sleep(5 * time.Second)
			_ = m.Start()
		}
	}()

	return nil
}

func (m *Manager) scanPipe(pipe io.Reader, label string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		m.BroadcastLog(line)
	}
	if err := scanner.Err(); err != nil {
		logger.Error("%s scanner error: %v", label, err)
	}
}

func (m *Manager) SendCommand(cmdStr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status != StatusRunning && m.status != StatusStarting {
		return fmt.Errorf("server is not running")
	}

	if m.stdin == nil {
		return fmt.Errorf("stdin pipe is nil")
	}

	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return nil
	}

	m.BroadcastLog(fmt.Sprintf("> %s", cmdStr))
	_, err := io.WriteString(m.stdin, cmdStr+"\n")
	return err
}

func (m *Manager) Stop() error {
	if m.GetStatus() == StatusStopped {
		return fmt.Errorf("server is already stopped")
	}

	m.SetStatus(StatusStopping)
	m.BroadcastLog("Sending /stop command to Minecraft server...")
	err := m.SendCommand("stop")
	if err != nil {
		m.BroadcastLog(fmt.Sprintf("Failed to send stop command: %v. Force killing...", err))
		return m.Kill()
	}

	// Wait up to 20s for graceful shutdown
	go func() {
		for i := 0; i < 20; i++ {
			time.Sleep(1 * time.Second)
			if m.GetStatus() == StatusStopped {
				return
			}
		}
		if m.GetStatus() != StatusStopped {
			m.BroadcastLog("Server did not stop gracefully within 20s. Force killing...")
			_ = m.Kill()
		}
	}()

	return nil
}

func (m *Manager) Restart() error {
	m.BroadcastLog("Initiating server restart...")
	if m.GetStatus() != StatusStopped {
		if err := m.Stop(); err != nil {
			return err
		}
		// Wait for stop
		for i := 0; i < 25; i++ {
			if m.GetStatus() == StatusStopped {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
	return m.Start()
}

func (m *Manager) Kill() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		err := m.cmd.Process.Kill()
		m.status = StatusStopped
		m.BroadcastLog("Process force killed.")
		return err
	}
	m.status = StatusStopped
	return nil
}

func ensureEula(serverDir string) {
	eulaPath := filepath.Join(serverDir, "eula.txt")
	content := "# Auto-accepted by VoidPanel\neula=true\n"
	_ = os.WriteFile(eulaPath, []byte(content), 0644)
}
