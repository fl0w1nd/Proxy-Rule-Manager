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
	Long:  "Full update (no args) or partial update (rule IDs + dependents).",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := buildApp()
		if err != nil {
			return err
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		req := updates.Request{Scope: "all"}
		if len(args) > 0 {
			req = updates.Request{Scope: "rules", RuleIDs: args}
		}
		record, err := app.Updates.Run(ctx, req, "cli")
		if err != nil {
			return err
		}

		fmt.Printf("Update complete: %d rules succeeded, %d failed, %d artifacts, %d changed\n",
			record.RulesSucceeded, record.RulesFailed, record.ArtifactsProcessed, len(record.Changes))

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
	rootCmd.AddCommand(updateCmd)
}
