package model

import "time"

const SchemaVersion = "1"

type Snapshot struct {
	CollectedAt time.Time      `json:"collected_at"`
	CPU         CPU            `json:"cpu"`
	Memory      Memory         `json:"memory"`
	Disks       []Disk         `json:"disks"`
	Filesystems []Filesystem   `json:"filesystems"`
	Networks    []Network      `json:"networks"`
	NUMA        NUMA           `json:"numa"`
	System      SystemSettings `json:"system"`
	Errors      []string       `json:"collector_errors"`
}

type CPU struct {
	LogicalCPUs   int64   `json:"logical_cpus"`
	UserPercent   float64 `json:"user_percent"`
	SystemPercent float64 `json:"system_percent"`
	IOWaitPercent float64 `json:"iowait_percent"`
	IdlePercent   float64 `json:"idle_percent"`
	Load1         float64 `json:"load_1m"`
	Load5         float64 `json:"load_5m"`
	Load15        float64 `json:"load_15m"`
	ContextSwitch int64   `json:"context_switches_per_second"`
	Interrupts    int64   `json:"interrupts_per_second"`
}

type Memory struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
	SwapTotalBytes uint64  `json:"swap_total_bytes"`
	SwapFreeBytes  uint64  `json:"swap_free_bytes"`
	SwapInPerSec   int64   `json:"swap_in_per_second"`
	SwapOutPerSec  int64   `json:"swap_out_per_second"`
}

type Disk struct {
	Name             string  `json:"name"`
	ReadBytes        uint64  `json:"read_bytes"`
	WriteBytes       uint64  `json:"write_bytes"`
	ReadsPerSec      float64 `json:"reads_per_second"`
	WritesPerSec     float64 `json:"writes_per_second"`
	ReadBytesPerSec  float64 `json:"read_bytes_per_second"`
	WriteBytesPerSec float64 `json:"write_bytes_per_second"`
	InFlight         int64   `json:"in_flight"`
}

type Network struct {
	Name          string  `json:"name"`
	State         string  `json:"state"`
	RXBytes       uint64  `json:"rx_bytes"`
	TXBytes       uint64  `json:"tx_bytes"`
	RXBytesPerSec float64 `json:"rx_bytes_per_second"`
	TXBytesPerSec float64 `json:"tx_bytes_per_second"`
	RXPackets     int64   `json:"rx_packets"`
	TXPackets     int64   `json:"tx_packets"`
	RXErrors      int64   `json:"rx_errors"`
	TXErrors      int64   `json:"tx_errors"`
	RXDrops       int64   `json:"rx_drops"`
	TXDrops       int64   `json:"tx_drops"`
	LinkSpeedMbps int64   `json:"link_speed_mbps,omitempty"`
	MTU           int64   `json:"mtu,omitempty"`
	RXQueues      int64   `json:"rx_queues,omitempty"`
	TXQueues      int64   `json:"tx_queues,omitempty"`
}

type Filesystem struct {
	Device         string  `json:"device"`
	MountPoint     string  `json:"mount_point"`
	Type           string  `json:"type"`
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
	ReadOnly       bool    `json:"read_only"`
}

type NUMA struct {
	Nodes        int   `json:"nodes"`
	RemoteEvents int64 `json:"remote_events_per_second"`
}

type SystemSettings struct {
	CPUGovernor  string `json:"cpu_governor,omitempty"`
	THP          string `json:"transparent_hugepages,omitempty"`
	Swappiness   int64  `json:"swappiness"`
	OpenFiles    uint64 `json:"open_files_limit"`
	MaxLocked    uint64 `json:"max_locked_memory_bytes"`
	MaxProcesses uint64 `json:"max_processes_limit"`
	MaxStack     uint64 `json:"max_stack_bytes"`
}

type Finding struct {
	Severity       string `json:"severity"`
	Category       string `json:"category"`
	Title          string `json:"title"`
	Evidence       string `json:"evidence"`
	Recommendation string `json:"recommendation"`
}

type Report struct {
	SchemaVersion string    `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	DurationMS    int64     `json:"duration_ms"`
	Snapshot      Snapshot  `json:"snapshot"`
	Findings      []Finding `json:"findings"`
}
