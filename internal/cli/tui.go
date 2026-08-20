package cli

import (
	"time"

	"github.com/spf13/cobra"
	"hardware-resources-tool/internal/analyze"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/tui"
)

func init() {
	var interval time.Duration
	thresholds := analyze.DefaultThresholds
	command := &cobra.Command{Use: "tui", Short: "Run the live terminal dashboard", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		if err := requireRoot(); err != nil {
			return err
		}
		return tui.Run(collect.New(), interval, thresholds)
	}}
	command.Flags().DurationVar(&interval, "interval", 2*time.Second, "refresh interval (minimum 500ms)")
	command.Flags().Float64Var(&thresholds.CPUIdleCritical, "cpu-idle-critical", thresholds.CPUIdleCritical, "critical finding below this idle CPU percentage")
	command.Flags().Float64Var(&thresholds.IOWaitWarning, "iowait-warning", thresholds.IOWaitWarning, "warning finding above this iowait percentage")
	command.Flags().Float64Var(&thresholds.MemoryUsedCritical, "memory-used-critical", thresholds.MemoryUsedCritical, "critical finding above this memory percentage")
	command.Flags().Float64Var(&thresholds.FilesystemUsedWarning, "filesystem-used-warning", thresholds.FilesystemUsedWarning, "warning finding above this filesystem percentage")
	command.Flags().Float64Var(&thresholds.FilesystemUsedCritical, "filesystem-used-critical", thresholds.FilesystemUsedCritical, "critical finding above this filesystem percentage")
	rootCmd.AddCommand(command)
}
