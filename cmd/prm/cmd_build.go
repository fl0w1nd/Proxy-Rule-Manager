package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

var buildOutput string

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build a standalone static rule site",
	RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := buildApp()
		if err != nil {
			return err
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		record, err := app.Updates.Run(ctx, updates.Request{Scope: "all"}, "cli")
		if err != nil {
			return err
		}
		for _, warning := range record.Warnings {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
		}
		if record.Status == "cancelled" {
			return fmt.Errorf("static build cancelled")
		}
		if len(record.Issues) > 0 {
			fmt.Fprintln(os.Stderr, "Errors:")
			for _, issue := range record.Issues {
				fmt.Fprintf(os.Stderr, "  - %s\n", issue.Message)
			}
			return fmt.Errorf("static build stopped after %d update errors", len(record.Issues))
		}
		if record.Status != "completed" && record.Status != "completed_with_warnings" {
			return fmt.Errorf("static build stopped after update status %q", record.Status)
		}

		if err := app.Engine.ExportStatic(buildOutput); err != nil {
			return fmt.Errorf("export static site: %w", err)
		}
		output, _ := filepath.Abs(buildOutput)
		fmt.Printf("Static site built: %s\n", output)
		return nil
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildOutput, "output", "dist", "standalone static output directory")
	rootCmd.AddCommand(buildCmd)
}
