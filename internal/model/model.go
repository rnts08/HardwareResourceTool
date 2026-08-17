package model

import "time"

const SchemaVersion = "1"

type Snapshot struct {
	CollectedAt         time.Time      `json:"collected_at"`
	CPU                 CPU            `json:"cpu"`
	Memory              Memory         `json:"memory"`
	Disks               []Disk         `json:"disks"`
	Filesystems         []Filesystem   `json:"filesystems"`
	Networks            []Network      `json:"networks"`
	VirtualNetworkCount int            `json:"virtual_network_count,omitempty"`
	PCI                 []PCIDevice    `json:"pci_devices"`
	MemoryDevices       []MemoryDevice `json:"memory_devices"`
	GPUs                []GPU          `json:"gpus"`
	Virtualization      Virtualization `json:"virtualization"`
	NUMA                NUMA           `json:"numa"`
	System              SystemSettings `json:"system"`
	Errors              []string       `json:"collector_errors"`
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
	Name                string            `json:"name"`
	State               string            `json:"state"`
	RXBytes             uint64            `json:"rx_bytes"`
	TXBytes             uint64            `json:"tx_bytes"`
	RXBytesPerSec       float64           `json:"rx_bytes_per_second"`
	TXBytesPerSec       float64           `json:"tx_bytes_per_second"`
	RXPackets           int64             `json:"rx_packets"`
	TXPackets           int64             `json:"tx_packets"`
	RXErrors            int64             `json:"rx_errors"`
	TXErrors            int64             `json:"tx_errors"`
	RXDrops             int64             `json:"rx_drops"`
	TXDrops             int64             `json:"tx_drops"`
	LinkSpeedMbps       int64             `json:"link_speed_mbps,omitempty"`
	MTU                 int64             `json:"mtu,omitempty"`
	RXQueues            int64             `json:"rx_queues,omitempty"`
	TXQueues            int64             `json:"tx_queues,omitempty"`
	RXRingSize          int64             `json:"rx_ring_size,omitempty"`
	TXRingSize          int64             `json:"tx_ring_size,omitempty"`
	Driver              string            `json:"driver,omitempty"`
	LinkDuplex          string            `json:"link_duplex,omitempty"`
	AutoNegotiation     string            `json:"autonegotiation,omitempty"`
	LinkUp              bool              `json:"link_up,omitempty"`
	SupportedLinkModes  []string          `json:"supported_link_modes,omitempty"`
	AdvertisedLinkModes []string          `json:"advertised_link_modes,omitempty"`
	PeerLinkModes       []string          `json:"peer_link_modes,omitempty"`
	FECActive           string            `json:"fec_active,omitempty"`
	FECSupported        string            `json:"fec_supported,omitempty"`
	MaxRXChannels       int64             `json:"max_rx_channels,omitempty"`
	MaxTXChannels       int64             `json:"max_tx_channels,omitempty"`
	MaxCombinedChannels int64             `json:"max_combined_channels,omitempty"`
	PauseAutoneg        bool              `json:"pause_autoneg,omitempty"`
	RXPause             bool              `json:"rx_pause,omitempty"`
	TXPause             bool              `json:"tx_pause,omitempty"`
	Timestamping        bool              `json:"timestamping,omitempty"`
	PHCIndex            int64             `json:"phc_index,omitempty"`
	EthtoolError        string            `json:"ethtool_error,omitempty"`
	DriverStats         map[string]uint64 `json:"driver_stats,omitempty"`
	Physical            bool              `json:"physical"`
	PCIAddress          string            `json:"pci_address,omitempty"`
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
	PCIeCapabilityMaxSpeed  string   `json:"pcie_capability_max_speed,omitempty"`
	PCIeCapabilityMaxWidth  int64    `json:"pcie_capability_max_width,omitempty"`
	PCIeNegotiatedSpeed     string   `json:"pcie_negotiated_speed,omitempty"`
	PCIeNegotiatedWidth     int64    `json:"pcie_negotiated_width,omitempty"`
	PCIePath                []string `json:"pcie_path,omitempty"`
	PCIePathMinSpeed        string   `json:"pcie_path_min_speed,omitempty"`
	PCIePathMinWidth        int64    `json:"pcie_path_min_width,omitempty"`
	PCIePathBandwidthGbps   float64  `json:"pcie_path_bandwidth_gbps,omitempty"`
	PCIePathBottleneck      string   `json:"pcie_path_bottleneck,omitempty"`
	PCIeParentAddress       string   `json:"pcie_parent_address,omitempty"`
	PCIePFAddress           string   `json:"pcie_pf_address,omitempty"`
	PCIeVFAddresses         []string `json:"pcie_vf_addresses,omitempty"`
	BARTotalBytes           uint64   `json:"bar_total_bytes,omitempty"`
	BARCount                int64    `json:"bar_count,omitempty"`
	BARAbove4G              bool     `json:"bar_above_4g,omitempty"`
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
	UUID               string  `json:"uuid,omitempty"`
	MemoryBytes        uint64  `json:"memory_bytes,omitempty"`
	UtilizationPercent float64 `json:"utilization_percent,omitempty"`
	TemperatureCelsius float64 `json:"temperature_celsius,omitempty"`
	PowerWatts         float64 `json:"power_watts,omitempty"`
	MemoryUsedBytes    uint64  `json:"memory_used_bytes,omitempty"`
	ECCEnabled         bool    `json:"ecc_enabled,omitempty"`
	ECCCorrected       uint64  `json:"ecc_corrected,omitempty"`
	ECCUncorrected     uint64  `json:"ecc_uncorrected,omitempty"`
	MIGEnabled         bool    `json:"mig_enabled,omitempty"`
	MIGMaxInstances    uint64  `json:"mig_max_instances,omitempty"`
	NVMLStatus         string  `json:"nvml_status,omitempty"`
	NVML               bool    `json:"nvml_available"`
}

type Virtualization struct {
	KVMAvailable          bool             `json:"kvm_available"`
	QEMUDetected          bool             `json:"qemu_detected"`
	Hypervisor            string           `json:"hypervisor,omitempty"`
	VirtualMachines       []VirtualMachine `json:"virtual_machines"`
	AllocatedVCPUs        int64            `json:"allocated_vcpus"`
	AllocatedMemoryBytes  uint64           `json:"allocated_memory_bytes"`
	VCPUOvercommitRatio   float64          `json:"vcpu_overcommit_ratio,omitempty"`
	MemoryOvercommitRatio float64          `json:"memory_overcommit_ratio,omitempty"`
}

type VirtualMachine struct {
	Name                  string         `json:"name"`
	PID                   int            `json:"pid,omitempty"`
	ConfiguredVCPUs       int64          `json:"configured_vcpus,omitempty"`
	ConfiguredMemoryBytes uint64         `json:"configured_memory_bytes,omitempty"`
	CPUPercent            float64        `json:"cpu_percent,omitempty"`
	CgroupCPUPercent      float64        `json:"cgroup_cpu_percent,omitempty"`
	MemoryCurrentBytes    uint64         `json:"memory_current_bytes,omitempty"`
	MemoryMaxBytes        uint64         `json:"memory_max_bytes,omitempty"`
	ProcessRSSBytes       uint64         `json:"process_rss_bytes,omitempty"`
	ReadBytes             uint64         `json:"read_bytes,omitempty"`
	WriteBytes            uint64         `json:"write_bytes,omitempty"`
	CgroupPath            string         `json:"cgroup_path,omitempty"`
	CgroupAvailable       bool           `json:"cgroup_available"`
	Hugepages             bool           `json:"hugepages"`
	HugepageBytes         uint64         `json:"hugepage_bytes,omitempty"`
	NUMANodes             []int          `json:"numa_nodes,omitempty"`
	BalloonEnabled        bool           `json:"balloon_enabled"`
	BalloonActualBytes    uint64         `json:"balloon_actual_bytes,omitempty"`
	BalloonTargetBytes    uint64         `json:"balloon_target_bytes,omitempty"`
	BalloonReclaimedBytes uint64         `json:"balloon_reclaimed_bytes,omitempty"`
	BalloonCommittedBytes uint64         `json:"balloon_committed_bytes,omitempty"`
	BalloonAvailableBytes uint64         `json:"balloon_available_bytes,omitempty"`
	BalloonReported       bool           `json:"balloon_reported"`
	BalloonGuestReport    bool           `json:"balloon_guest_report"`
	BalloonSource         string         `json:"balloon_source,omitempty"`
	QMPVersion            string         `json:"qmp_version,omitempty"`
	QMPBaseMemoryBytes    uint64         `json:"qmp_base_memory_bytes,omitempty"`
	QMPPluggedMemoryBytes uint64         `json:"qmp_plugged_memory_bytes,omitempty"`
	QMPVCPUs              int64          `json:"qmp_vcpus,omitempty"`
	QMPEnabledVCPUs       int64          `json:"qmp_enabled_vcpus,omitempty"`
	RuntimeAnonHugeBytes  uint64         `json:"runtime_anon_huge_bytes,omitempty"`
	RuntimeHugetlbBytes   uint64         `json:"runtime_hugetlb_bytes,omitempty"`
	RuntimeNUMABytes      map[int]uint64 `json:"runtime_numa_bytes,omitempty"`
	QMPStatus             string         `json:"qmp_status,omitempty"`
	Disks                 []VirtualDisk  `json:"disks,omitempty"`
	NICs                  []VirtualNIC   `json:"nics,omitempty"`
	PCIAddresses          []string       `json:"pci_addresses,omitempty"`
	Running               bool           `json:"running"`
	Source                string         `json:"source,omitempty"`
}

type VirtualDisk struct {
	Target string `json:"target,omitempty"`
	Source string `json:"source,omitempty"`
	Bus    string `json:"bus,omitempty"`
}

type VirtualNIC struct {
	Type             string  `json:"type,omitempty"`
	Source           string  `json:"source,omitempty"`
	Target           string  `json:"target,omitempty"`
	MAC              string  `json:"mac,omitempty"`
	HostNetwork      string  `json:"host_network,omitempty"`
	RXBytesPerSecond float64 `json:"rx_bytes_per_second,omitempty"`
	TXBytesPerSecond float64 `json:"tx_bytes_per_second,omitempty"`
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
