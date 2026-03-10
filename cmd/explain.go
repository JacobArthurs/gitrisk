/*
Copyright © 2026 JACOB ARTHURS
*/
package cmd

import (
	"fmt"

	"github.com/jacobarthurs/gitrisk/internal/config"
	"github.com/jacobarthurs/gitrisk/internal/git"
	"github.com/jacobarthurs/gitrisk/internal/output"
	"github.com/jacobarthurs/gitrisk/internal/score"
	"github.com/spf13/cobra"
)

var explainCmd = &cobra.Command{
	Use:   "explain <file>",
	Short: "Show full signal breakdown for a single file",
	Args:  cobra.ExactArgs(1),
	RunE:  runExplain,
}

func runExplain(cmd *cobra.Command, args []string) error {
	file := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	signals, err := git.CollectSignals([]string{file}, cfg)
	if err != nil {
		return fmt.Errorf("collecting signals: %w", err)
	}

	norms, err := git.ComputeNorms(cfg)
	if err != nil {
		return fmt.Errorf("computing norms: %w", err)
	}

	results := score.ScoreAll(signals, norms, cfg, false)
	if len(results) == 0 {
		return fmt.Errorf("no data for file: %s", file)
	}

	output.RenderExplain(results[0], norms, cfg.LookbackDays)
	return nil
}
