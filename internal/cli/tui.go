package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"hardware-resources-tool/internal/analyze"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/tui"
)

func init() {
	var interval time.Duration
	thresholds := analyze.DefaultThresholds
	command := &cobra.Command{Use: "tui", Short: "Run the live terminal dashboard", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := requireRoot(); err != nil {
			return err
		}
		reportText, err := tui.Run(collect.New(), interval, thresholds)
		if reportText != "" {
			fmt.Fprintln(cmd.OutOrStdout(), reportText)
		}
		return err
	}}
	command.Flags().DurationVar(&interval, "interval", 2*time.Second, "refresh interval (minimum 500ms)")
	command.Flags().Float64Var(&thresholds.CPUIdleCritical, "cpu-idle-critical", thresholds.CPUIdleCritical, "critical finding below this idle CPU percentage")
	command.Flags().Float64Var(&thresholds.IOWaitWarning, "iowait-warning", thresholds.IOWaitWarning, "warning finding above this iowait percentage")
	command.Flags().Float64Var(&thresholds.MemoryUsedCritical, "memory-used-critical", thresholds.MemoryUsedCritical, "critical finding above this memory percentage")
	command.Flags().Float64Var(&thresholds.FilesystemUsedWarning, "filesystem-used-warning", thresholds.FilesystemUsedWarning, "warning finding above this filesystem percentage")
	command.Flags().Float64Var(&thresholds.FilesystemUsedCritical, "filesystem-used-critical", thresholds.FilesystemUsedCritical, "critical finding above this filesystem percentage")
	rootCmd.AddCommand(command)
}
