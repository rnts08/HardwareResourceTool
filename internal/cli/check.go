package cli

import (
	"github.com/spf13/cobra"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/report"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "check", Short: "Run a one-shot host check", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if err := requireRoot(); err != nil {
			return err
		}
		result := report.Collect(collect.New(), 0)
		return report.WriteText(cmd.OutOrStdout(), result)
	}})
}
