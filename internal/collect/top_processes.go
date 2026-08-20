package collect

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hardware-resources-tool/internal/model"
)

const topProcessLimit = 10

// collectTopProcesses samples the highest CPU consumers among all host
// processes. Per-process jiffies deltas between snapshots drive the CPU rate,
// mirroring the QEMU process accounting used for virtual machines.
func (c *Collector) collectTopProcesses(s *model.Snapshot, raw *rawCounters, seconds float64) error {
	entries, err := os.ReadDir(c.procRoot)
	if err != nil {
		return err
	}
	pageSize := uint64(os.Getpagesize())
	type processSample struct {
		model.ProcessSample
		pid uint64
	}
	candidates := make([]processSample, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		base := filepath.Join(c.procRoot, entry.Name())
		comm, err := os.ReadFile(filepath.Join(base, "comm"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(comm))
		if name == "" {
			continue
		}
		stat, err := os.ReadFile(filepath.Join(base, "stat"))
		if err != nil {
			continue
		}
		jiffies := parseProcessJiffies(string(stat))
		cpu := float64(0)
		if previous, ok := raw.processes[pid]; ok && seconds > 0 && jiffies >= previous {
			cpu = float64(jiffies-previous) / (seconds * 100.0) * 100
		}
		raw.processes[pid] = jiffies
		sample := processSample{ProcessSample: model.ProcessSample{PID: pid, Name: name, CPUPercent: cpu}, pid: uint64(pid)}
		if statm, readErr := os.ReadFile(filepath.Join(base, "statm")); readErr == nil {
			values := strings.Fields(string(statm))
			if len(values) > 1 {
				if pages, parseErr := strconv.ParseUint(values[1], 10, 64); parseErr == nil {
					sample.RSSBytes = pages * pageSize
				}
			}
		}
		if fields := strings.Fields(string(stat)); len(fields) > 2 {
			sample.State = fields[2]
		}
		candidates = append(candidates, sample)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CPUPercent != candidates[j].CPUPercent {
			return candidates[i].CPUPercent > candidates[j].CPUPercent
		}
		return candidates[i].RSSBytes > candidates[j].RSSBytes
	})
	if len(candidates) > topProcessLimit {
		candidates = candidates[:topProcessLimit]
	}
	s.TopProcesses = make([]model.ProcessSample, 0, len(candidates))
	for _, sample := range candidates {
		s.TopProcesses = append(s.TopProcesses, sample.ProcessSample)
	}
	return nil
}
