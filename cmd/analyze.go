/*
Copyright © 2026 JACOB ARTHURS
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/jacobarthurs/gitrisk/internal/config"
	"github.com/jacobarthurs/gitrisk/internal/git"
	"github.com/jacobarthurs/gitrisk/internal/output"
	"github.com/jacobarthurs/gitrisk/internal/score"
	"github.com/spf13/cobra"
)

var (
	flagBase      string
	flagHead      string
	flagStaged    bool
	flagCoupling  bool
	flagFormat    string
	flagThreshold string
	flagExitCode  bool
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze risk of changed files",
	RunE:  runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVar(&flagBase, "base", "main", "Base ref for diff")
	analyzeCmd.Flags().StringVar(&flagHead, "head", "HEAD", "Head ref for diff")
	analyzeCmd.Flags().BoolVar(&flagStaged, "staged", false, "Analyze staged changes only")
	analyzeCmd.Flags().BoolVar(&flagCoupling, "coupling", false, "Include change coupling analysis (slower)")
	analyzeCmd.Flags().StringVar(&flagFormat, "format", "table", "Output format: table, json")
	analyzeCmd.Flags().StringVar(&flagThreshold, "threshold", "low", "Minimum risk level to report: low, medium, high, critical")
	analyzeCmd.Flags().BoolVar(&flagExitCode, "exit-code", false, "Return exit code 1 if any file meets or exceeds threshold")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var changedFiles []string
	if flagStaged {
		changedFiles, err = git.StagedFiles()
	} else {
		changedFiles, err = git.DiffFiles(flagBase, flagHead)
	}
	if err != nil {
		return fmt.Errorf("getting diff: %w", err)
	}
	if len(changedFiles) == 0 {
		fmt.Println("No changed files found.")
		return nil
	}

	changedFiles = config.FilterIgnored(changedFiles, cfg.Ignore)

	signals, err := git.CollectSignals(changedFiles, cfg)
	if err != nil {
		return fmt.Errorf("collecting signals: %w", err)
	}

	norms, err := git.ComputeNorms(cfg)
	if err != nil {
		return fmt.Errorf("computing norms: %w", err)
	}

	results := score.ScoreAll(signals, norms, cfg, flagCoupling)

	var couplingWarnings []git.CouplingWarning
	if flagCoupling {
		couplingWarnings, err = git.CouplingWarnings(changedFiles, cfg)
		if err != nil {
			return fmt.Errorf("computing coupling: %w", err)
		}
	}

	minLevel := score.ParseLevel(flagThreshold)
	results = score.FilterByLevel(results, minLevel)

	renderer := output.NewRenderer(flagFormat)
	if err := renderer.Render(results, couplingWarnings); err != nil {
		return err
	}

	if flagExitCode && score.AnyMeetsLevel(results, minLevel) {
		os.Exit(1)
	}

	return nil
}
