package cli

import (
	"time"

	"github.com/spf13/cobra"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/tui"
)

func init() {
	var interval time.Duration
	command := &cobra.Command{Use: "tui", Short: "Run the live terminal dashboard", Args: cobra.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		if err := requireRoot(); err != nil {
			return err
		}
		return tui.Run(collect.New(), interval)
	}}
	command.Flags().DurationVar(&interval, "interval", 2*time.Second, "refresh interval (minimum 500ms)")
	rootCmd.AddCommand(command)
}
