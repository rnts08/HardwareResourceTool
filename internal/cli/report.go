package cli

import (
	"time"

	"github.com/spf13/cobra"
	"hardware-resources-tool/internal/analyze"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/report"
)

func init() {
	var duration time.Duration
	var jsonOutput bool
	thresholds := analyze.DefaultThresholds
	command := &cobra.Command{Use: "report", Short: "Collect and print a host report", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := requireRoot(); err != nil {
			return err
		}
		result := report.CollectWithThresholds(collect.New(), duration, thresholds)
		if jsonOutput {
			return report.WriteJSON(cmd.OutOrStdout(), result)
		}
		return report.WriteText(cmd.OutOrStdout(), result)
	}}
	command.Flags().DurationVar(&duration, "duration", time.Second, "time between the initial and final samples")
	command.Flags().BoolVar(&jsonOutput, "json", false, "write a machine-readable JSON report")
	command.Flags().Float64Var(&thresholds.CPUIdleCritical, "cpu-idle-critical", thresholds.CPUIdleCritical, "critical finding below this idle CPU percentage")
	command.Flags().Float64Var(&thresholds.IOWaitWarning, "iowait-warning", thresholds.IOWaitWarning, "warning finding above this iowait percentage")
	command.Flags().Float64Var(&thresholds.MemoryUsedCritical, "memory-used-critical", thresholds.MemoryUsedCritical, "critical finding above this memory percentage")
	command.Flags().Float64Var(&thresholds.FilesystemUsedWarning, "filesystem-used-warning", thresholds.FilesystemUsedWarning, "warning finding above this filesystem percentage")
	command.Flags().Float64Var(&thresholds.FilesystemUsedCritical, "filesystem-used-critical", thresholds.FilesystemUsedCritical, "critical finding above this filesystem percentage")
	rootCmd.AddCommand(command)
}
