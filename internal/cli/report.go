package cli

import (
	"time"

	"github.com/spf13/cobra"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/report"
)

func init() {
	var duration time.Duration
	var jsonOutput bool
	command := &cobra.Command{Use: "report", Short: "Collect and print a host report", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := requireRoot(); err != nil {
			return err
		}
		result := report.Collect(collect.New(), duration)
		if jsonOutput {
			return report.WriteJSON(cmd.OutOrStdout(), result)
		}
		return report.WriteText(cmd.OutOrStdout(), result)
	}}
	command.Flags().DurationVar(&duration, "duration", time.Second, "time between the initial and final samples")
	command.Flags().BoolVar(&jsonOutput, "json", false, "write a machine-readable JSON report")
	rootCmd.AddCommand(command)
}
