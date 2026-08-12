// Command prm is the CLI entry point for Proxy Rule Manager.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fl0w1nd/proxy-rule-manager/version"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:     "prm",
	Short:   "Proxy Rule Manager — CLI-first rule orchestration",
	Long:    "Proxy Rule Manager compiles proxy routing rules from diverse upstream sources into client-specific formats.",
	Version: version.Current(),
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.yaml", "config file path")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
