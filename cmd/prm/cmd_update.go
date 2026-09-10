package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

var updateCmd = &cobra.Command{
	Use:   "update [rule-ids...]",
	Short: "Compile rules and write artifacts",
	Long:  "Full update (no args), partial update (rule IDs + dependents), or refresh a Geo database and its published artifacts (--geosite or --geoip).",
	Args: func(cmd *cobra.Command, args []string) error {
		geosite, _ := cmd.Flags().GetBool("geosite")
		geoip, _ := cmd.Flags().GetBool("geoip")
		if len(args) > 0 && (geosite || geoip) {
			return fmt.Errorf("rule IDs cannot be combined with --geosite or --geoip")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, err := resolveDataDir(cmd)
		if err != nil {
			return err
		}
		app, err := buildApp(dataDir)
		if err != nil {
			return err
		}
		defer app.Close()

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		req := updates.Request{Scope: "all"}
		if len(args) > 0 {
			req = updates.Request{Scope: "rules", RuleIDs: args}
		}
		if geosite, _ := cmd.Flags().GetBool("geosite"); geosite {
			req.Scope = "geosite"
		}
		if geoip, _ := cmd.Flags().GetBool("geoip"); geoip {
			req.Scope = "geoip"
		}
		record, err := app.Updates.Run(ctx, req, "cli")
		if err != nil {
			return err
		}

		if req.Scope == "geosite" || req.Scope == "geoip" {
			fmt.Printf("%s update complete: %d artifacts\n", req.Scope, record.ArtifactsProcessed)
		} else {
			fmt.Printf("Update complete: %d rules succeeded, %d failed, %d artifacts, %d changed\n",
				record.RulesSucceeded, record.RulesFailed, record.ArtifactsProcessed, len(record.Changes))
		}

		if len(record.Issues) > 0 {
			fmt.Fprintln(os.Stderr, "\nErrors:")
			for _, issue := range record.Issues {
				fmt.Fprintf(os.Stderr, "  - %s\n", issue.Message)
			}
			return fmt.Errorf("%d errors occurred", len(record.Issues))
		}
		return nil
	},
}

func init() {
	updateCmd.Flags().Bool("geosite", false, "Update Geosite databases and published artifacts")
	updateCmd.Flags().Bool("geoip", false, "Update GeoIP databases and published artifacts")
	updateCmd.MarkFlagsMutuallyExclusive("geosite", "geoip")
	rootCmd.AddCommand(updateCmd)
}
