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

var hotspotsCmd = &cobra.Command{
	Use:   "hotspots",
	Short: "Show top 10 riskiest files repo-wide (not PR-scoped)",
	RunE:  runHotspots,
}

func runHotspots(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	allFiles, err := git.AllTouchedFiles(cfg)
	if err != nil {
		return fmt.Errorf("getting files: %w", err)
	}

	signals, err := git.CollectSignals(allFiles, cfg)
	if err != nil {
		return fmt.Errorf("collecting signals: %w", err)
	}

	norms, err := git.ComputeNorms(cfg)
	if err != nil {
		return fmt.Errorf("computing norms: %w", err)
	}

	results := score.ScoreAll(signals, norms, cfg, false)
	results = score.TopN(results, 10)

	renderer := output.NewRenderer("table")
	return renderer.Render(results, nil)
}
