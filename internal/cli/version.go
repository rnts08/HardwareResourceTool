package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Release builds can set all three values with -ldflags.
var version = "0.2.3"
var buildCommit = "unknown"
var buildDate = "unknown"

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "version", Short: "Print version information", Args: cobra.NoArgs, Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "hardware-resources %s (%s/%s) commit=%s built=%s\n", version, runtime.GOOS, runtime.GOARCH, buildCommit, buildDate)
	}})
}
