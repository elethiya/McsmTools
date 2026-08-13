package metrics

import (
	"fmt"
	"math"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"

	"McsmTools/pkg/config"
	"McsmTools/pkg/mcserver"
)

type SystemMetrics struct {
	CPUPercent     float64 `json:"cpu_percent"`
	RAMUsed        uint64  `json:"ram_used"`
	RAMTotal       uint64  `json:"ram_total"`
	RAMPercent     float64 `json:"ram_percent"`
	DiskUsed       uint64  `json:"disk_used"`
	DiskTotal      uint64  `json:"disk_total"`
	DiskPercent    float64 `json:"disk_percent"`
	ProcessCPU     float64 `json:"process_cpu"`
	ProcessRAM     uint64  `json:"process_ram"`
	ServerStatus   string  `json:"server_status"`
	UptimeSeconds  int64   `json:"uptime_seconds"`
	FormattedTime  string  `json:"formatted_uptime"`
}

func CollectMetrics() (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		ServerStatus: string(mcserver.GetManager().GetStatus()),
	}

	// System Memory
	vMem, err := mem.VirtualMemory()
	if err == nil {
		metrics.RAMUsed = vMem.Used
		metrics.RAMTotal = vMem.Total
		metrics.RAMPercent = roundTwoDecimals(vMem.UsedPercent)
	}

	// CPU %
	cpuPerc, err := cpu.Percent(0, false)
	if err == nil && len(cpuPerc) > 0 {
		metrics.CPUPercent = roundTwoDecimals(cpuPerc[0])
	}

	// Disk Stats
	cfg := config.GlobalConfig
	serverDir := "./"
	if cfg != nil {
		serverDir = cfg.ServerDir
	}
	dStat, err := disk.Usage(serverDir)
	if err == nil {
		metrics.DiskUsed = dStat.Used
		metrics.DiskTotal = dStat.Total
		metrics.DiskPercent = roundTwoDecimals(dStat.UsedPercent)
	}

	// MC Process Stats
	pid := mcserver.GetManager().GetPID()
	if pid > 0 {
		proc, err := process.NewProcess(int32(pid))
		if err == nil {
			pCPU, err := proc.CPUPercent()
			if err == nil {
				metrics.ProcessCPU = roundTwoDecimals(pCPU)
			}
			mInfo, err := proc.MemoryInfo()
			if err == nil && mInfo != nil {
				metrics.ProcessRAM = mInfo.RSS
			}
		}
	}

	uptime := mcserver.GetManager().GetUptime()
	metrics.UptimeSeconds = int64(uptime.Seconds())
	metrics.FormattedTime = formatDuration(uptime)

	return metrics, nil
}

func roundTwoDecimals(val float64) float64 {
	return math.Round(val*100) / 100
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
