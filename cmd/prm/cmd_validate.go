package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/fl0w1nd/proxy-rule-manager/internal/config"
	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config, templates, and geo references",
	Long: `Validate checks the config file for structural correctness, verifies
all client templates exist, and loads geosite and geoip provider caches to validate
list references.

Every error is reported with its YAML line number and config path.
Geo validation issues are reported individually without blocking
other rules from working.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, err := resolveDataDir(cmd)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		app, err := buildApp(dataDir)
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
			cmd.Context(),
			app.Config,
			app.Engine.Geosite,
			app.Logger,
		)

		diags = append(diags, engine.ValidateGeoIPRefs(cmd.Context(), app.Config, app.Engine.GeoIP, app.Logger)...)

		if len(diags) == 0 {
			fmt.Println("Geo references: all valid.")
			return nil
		}

		fmt.Fprintf(os.Stderr, "\nGeo validation issues:\n")
		printConfigErrors(diags)
		return fmt.Errorf("geo validation found %d issue(s)", len(diags))
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
