package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fl0w1nd/proxy-rule-manager/internal/engine"
	"github.com/fl0w1nd/proxy-rule-manager/internal/ir"
)

var previewTarget string

var previewCmd = &cobra.Command{
	Use:   "preview <rule-id>",
	Short: "Compile a rule and show per-stage report",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, err := resolveDataDir(cmd)
		if err != nil {
			return err
		}
		app, err := buildApp(dataDir)
		if err != nil {
			return err
		}

		report, err := engine.Preview(
			context.Background(),
			app.Config,
			app.DataDir,
			args[0],
			previewTarget,
			app.Registry,
			app.Engine.Fetcher,
			app.Engine.Preprocessor,
			app.Engine.Geosite,
			app.Logger,
		)
		if err != nil {
			return err
		}

		printPreviewReport(report)
		return nil
	},
}

func printPreviewReport(report *engine.PreviewReport) {
	fmt.Printf("Rule: %s\nID: %s\n\n", report.RuleName, report.RuleID)

	fmt.Println("=== Sources ===")
	for _, src := range report.Sources {
		if src.Error != "" {
			fmt.Printf("  [%s] ERROR: %s\n", src.Label, src.Error)
		} else {
			fmt.Printf("  [%s] %d entries, %d diagnostics\n", src.Label, len(src.Entries), len(src.Diagnostics))
		}
	}

	fmt.Printf("\n=== Pre-ops: %d entries ===\n", len(report.PreOps))
	printKindSummary(report.PreOps)

	fmt.Printf("\n=== Post-ops: %d entries ===\n", len(report.PostOps))
	printKindSummary(report.PostOps)

	fmt.Printf("\n=== Merged: %d entries ===\n", len(report.Merged))
	printKindSummary(report.Merged)
	if report.OpsError != "" {
		fmt.Printf("\n=== Ops Error ===\n%s\n", report.OpsError)
	}

	if report.RenderedOutput != nil {
		fmt.Printf("\n=== Rendered (%s) ===\n", report.RenderedTarget)
		_, _ = os.Stdout.Write(report.RenderedOutput)
	}
	if report.RenderError != "" {
		fmt.Printf("\n=== Render Error ===\n%s\n", report.RenderError)
	}
}

func printKindSummary(entries []ir.Entry) {
	counts := ir.CountKinds(entries)
	for _, kc := range counts {
		fmt.Printf("  %s: %d\n", kc.Kind, kc.Count)
	}
}

func init() {
	previewCmd.Flags().StringVar(&previewTarget, "target", "", "render output for an explicit format or variant target")
	rootCmd.AddCommand(previewCmd)
}
