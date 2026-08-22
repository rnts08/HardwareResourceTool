package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"hardware-resources-tool/internal/analyze"
	"hardware-resources-tool/internal/collect"
	"hardware-resources-tool/internal/model"
	"hardware-resources-tool/internal/report"
)

// checkExitCode maps a report to the documented `check` exit status:
//
//	0  PASS             no findings above info severity, no collector errors
//	1  WARNING          one or more warning-severity findings
//	2  CRITICAL         one or more critical-severity findings
//	3  COLLECTION ERROR collector errors present; the capture is incomplete
//
// Precedence is 3 > 2 > 1 > 0: an untrusted capture outranks any finding.
func checkExitCode(result model.Report) (int, string) {
	criticals, warnings := 0, 0
	for _, finding := range result.Findings {
		switch finding.Severity {
		case "critical":
			criticals++
		case "warning":
			warnings++
		}
	}
	if errors := len(result.Snapshot.Errors); errors > 0 {
		return 3, fmt.Sprintf("COLLECTION ERRORS (%d collector errors, %d critical, %d warning findings)", errors, criticals, warnings)
	}
	switch {
	case criticals > 0:
		return 2, fmt.Sprintf("CRITICAL (%d critical, %d warning findings)", criticals, warnings)
	case warnings > 0:
		return 1, fmt.Sprintf("WARNING (%d warning findings)", warnings)
	default:
		return 0, fmt.Sprintf("PASS (%d info findings)", len(result.Findings)-criticals-warnings)
	}
}

func init() {
	thresholds := analyze.DefaultThresholds
	var noFail bool
	command := &cobra.Command{
		Use:   "check",
		Short: "Run a one-shot host check",
		Long: "Run a one-shot host health check and print the text report.\n\n" +
			"Exit codes:\n" +
			"  0  PASS              no warning or critical findings, no collector errors\n" +
			"  1  WARNING           at least one warning-severity finding\n" +
			"  2  CRITICAL          at least one critical-severity finding\n" +
			"  3  COLLECTION ERRORS collector errors present, capture may be incomplete\n\n" +
			"Use --no-fail to always exit 0, for example when the output is piped.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireRoot(); err != nil {
				return err
			}
			result := report.CollectWithThresholds(collect.New(), time.Second, thresholds)
			if err := report.WriteText(cmd.OutOrStdout(), result); err != nil {
				return err
			}
			code, summary := checkExitCode(result)
			fmt.Fprintf(cmd.OutOrStdout(), "Result: %s\n", summary)
			if code != 0 && !noFail {
				os.Exit(code)
			}
			return nil
		},
	}
	command.Flags().Float64Var(&thresholds.CPUIdleCritical, "cpu-idle-critical", thresholds.CPUIdleCritical, "critical finding below this idle CPU percentage")
	command.Flags().Float64Var(&thresholds.IOWaitWarning, "iowait-warning", thresholds.IOWaitWarning, "warning finding above this iowait percentage")
	command.Flags().Float64Var(&thresholds.MemoryUsedCritical, "memory-used-critical", thresholds.MemoryUsedCritical, "critical finding above this memory percentage")
	command.Flags().Float64Var(&thresholds.FilesystemUsedWarning, "filesystem-used-warning", thresholds.FilesystemUsedWarning, "warning finding above this filesystem percentage")
	command.Flags().Float64Var(&thresholds.FilesystemUsedCritical, "filesystem-used-critical", thresholds.FilesystemUsedCritical, "critical finding above this filesystem percentage")
	command.Flags().BoolVar(&noFail, "no-fail", false, "always exit 0 regardless of findings or collector errors")
	rootCmd.AddCommand(command)
}
