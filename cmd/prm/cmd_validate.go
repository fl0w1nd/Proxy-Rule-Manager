package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config, templates, and geosite references",
	Long: `Validate checks the config file for structural correctness, verifies
all client templates exist, and loads geosite provider caches to validate
list and attr references.

Every error is reported with its YAML line number and config path.
Geosite validation issues are reported individually without blocking
other rules from working.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := buildApp()
		if err != nil {
			// buildApp already reports line-precise errors via ConfigErrors
			var errs config.ConfigErrors
			if errors.As(err, &errs) {
				printConfigErrors(errs)
				return fmt.Errorf("config validation failed with %d error(s)", len(errs))
			}
			return fmt.Errorf("validation failed: %w", err)
		}

		fmt.Println("Config and templates are valid.")

		// Deep geosite reference validation
		diags := engine.ValidateGeositeRefs(
			context.Background(),
			app.Config,
			app.Engine.Geosite,
			app.Logger,
		)

		if len(diags) == 0 {
			fmt.Println("Geosite references: all valid.")
			return nil
		}

		fmt.Fprintf(os.Stderr, "\nGeosite validation issues:\n")
		printConfigErrors(diags)
		return fmt.Errorf("geosite validation found %d issue(s)", len(diags))
	},
}

func printConfigErrors(errs []config.ConfigError) {
	for _, e := range errs {
		if e.Line > 0 {
			fmt.Fprintf(os.Stderr, "  line %d | %s: %s\n", e.Line, e.Path, e.Message)
		} else {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", e.Path, e.Message)
		}
	}
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
