package model

import "time"

const SchemaVersion = "1"

type Snapshot struct {
	CollectedAt   time.Time      `json:"collected_at"`
	CPU           CPU            `json:"cpu"`
	Memory        Memory         `json:"memory"`
	Disks         []Disk         `json:"disks"`
	Filesystems   []Filesystem   `json:"filesystems"`
	Networks      []Network      `json:"networks"`
	PCI           []PCIDevice    `json:"pci_devices"`
	MemoryDevices []MemoryDevice `json:"memory_devices"`
	GPUs          []GPU          `json:"gpus"`
	NUMA          NUMA           `json:"numa"`
	System        SystemSettings `json:"system"`
	Errors        []string       `json:"collector_errors"`
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
	Name                string   `json:"name"`
	State               string   `json:"state"`
	RXBytes             uint64   `json:"rx_bytes"`
	TXBytes             uint64   `json:"tx_bytes"`
	RXBytesPerSec       float64  `json:"rx_bytes_per_second"`
	TXBytesPerSec       float64  `json:"tx_bytes_per_second"`
	RXPackets           int64    `json:"rx_packets"`
	TXPackets           int64    `json:"tx_packets"`
	RXErrors            int64    `json:"rx_errors"`
	TXErrors            int64    `json:"tx_errors"`
	RXDrops             int64    `json:"rx_drops"`
	TXDrops             int64    `json:"tx_drops"`
	LinkSpeedMbps       int64    `json:"link_speed_mbps,omitempty"`
	MTU                 int64    `json:"mtu,omitempty"`
	RXQueues            int64    `json:"rx_queues,omitempty"`
	TXQueues            int64    `json:"tx_queues,omitempty"`
	RXRingSize          int64    `json:"rx_ring_size,omitempty"`
	TXRingSize          int64    `json:"tx_ring_size,omitempty"`
	Driver              string   `json:"driver,omitempty"`
	LinkDuplex          string   `json:"link_duplex,omitempty"`
	AutoNegotiation     string   `json:"autonegotiation,omitempty"`
	LinkUp              bool     `json:"link_up,omitempty"`
	SupportedLinkModes  []string `json:"supported_link_modes,omitempty"`
	AdvertisedLinkModes []string `json:"advertised_link_modes,omitempty"`
	PeerLinkModes       []string `json:"peer_link_modes,omitempty"`
	FECActive           string   `json:"fec_active,omitempty"`
	FECSupported        string   `json:"fec_supported,omitempty"`
	EthtoolError        string   `json:"ethtool_error,omitempty"`
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

type PCIDevice struct {
	Address                 string   `json:"address"`
	VendorID                string   `json:"vendor_id"`
	DeviceID                string   `json:"device_id"`
	Class                   string   `json:"class"`
	Driver                  string   `json:"driver,omitempty"`
	NUMANode                int64    `json:"numa_node"`
	IOMMUGroup              string   `json:"iommu_group,omitempty"`
	CurrentLinkSpeed        string   `json:"current_link_speed,omitempty"`
	CurrentLinkWidth        int64    `json:"current_link_width,omitempty"`
	MaxLinkSpeed            string   `json:"max_link_speed,omitempty"`
	MaxLinkWidth            int64    `json:"max_link_width,omitempty"`
	Capabilities            []string `json:"capabilities,omitempty"`
	PCIeMaxPayloadBytes     int64    `json:"pcie_max_payload_bytes,omitempty"`
	PCIeMaxReadRequestBytes int64    `json:"pcie_max_read_request_bytes,omitempty"`
	AERUncorrectableStatus  uint32   `json:"aer_uncorrectable_status,omitempty"`
	AERCorrectableStatus    uint32   `json:"aer_correctable_status,omitempty"`
	SRIOVTotalVFs           int64    `json:"sriov_total_vfs,omitempty"`
	ResizableBAR            bool     `json:"resizable_bar,omitempty"`
}

type GPU struct {
	Address            string  `json:"address"`
	VendorID           string  `json:"vendor_id"`
	DeviceID           string  `json:"device_id"`
	Name               string  `json:"name,omitempty"`
	MemoryBytes        uint64  `json:"memory_bytes,omitempty"`
	UtilizationPercent float64 `json:"utilization_percent,omitempty"`
	TemperatureCelsius float64 `json:"temperature_celsius,omitempty"`
	NVML               bool    `json:"nvml_available"`
}

type MemoryDevice struct {
	Locator            string `json:"locator,omitempty"`
	Manufacturer       string `json:"manufacturer,omitempty"`
	PartNumber         string `json:"part_number,omitempty"`
	Serial             string `json:"serial,omitempty"`
	Type               string `json:"type,omitempty"`
	SizeBytes          uint64 `json:"size_bytes"`
	SpeedMTs           uint64 `json:"speed_mts,omitempty"`
	ConfiguredSpeedMTs uint64 `json:"configured_speed_mts,omitempty"`
	CorrectedErrors    uint64 `json:"corrected_errors,omitempty"`
	UncorrectedErrors  uint64 `json:"uncorrected_errors,omitempty"`
}

type NUMA struct {
	Nodes        int   `json:"nodes"`
	RemoteEvents int64 `json:"remote_events_per_second"`
}

type SystemSettings struct {
	CPUGovernor  string            `json:"cpu_governor,omitempty"`
	THP          string            `json:"transparent_hugepages,omitempty"`
	Swappiness   int64             `json:"swappiness"`
	OpenFiles    uint64            `json:"open_files_limit"`
	MaxLocked    uint64            `json:"max_locked_memory_bytes"`
	MaxProcesses uint64            `json:"max_processes_limit"`
	MaxStack     uint64            `json:"max_stack_bytes"`
	Sysctls      map[string]string `json:"sysctls,omitempty"`
	HostLimits   Limits            `json:"host_limits"`
}

type Limits struct {
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
