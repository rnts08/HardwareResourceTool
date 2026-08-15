package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var version = "dev"

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "version", Short: "Print version information", Args: cobra.NoArgs, Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "hardware-resources %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	}})
}
