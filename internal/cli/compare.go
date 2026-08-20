package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"hardware-resources-tool/internal/model"
	"hardware-resources-tool/internal/report"
)

func init() {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "compare OLDER.json NEWER.json",
		Short: "Compare two saved JSON reports",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			older, err := readReportFile(args[0])
			if err != nil {
				return fmt.Errorf("reading %s: %w", args[0], err)
			}
			newer, err := readReportFile(args[1])
			if err != nil {
				return fmt.Errorf("reading %s: %w", args[1], err)
			}
			comparison := report.Compare(older, newer)
			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(comparison)
			}
			return report.WriteComparison(cmd.OutOrStdout(), comparison)
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "write the comparison as machine-readable JSON")
	rootCmd.AddCommand(command)
}

func readReportFile(path string) (result model.Report, err error) {
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()
	result, err = report.ReadReport(file)
	return result, err
}
