/*
Copyright © 2026 JACOB ARTHURS
*/
package cmd

import (
	"fmt"

	"github.com/jacobarthurs/gitrisk/internal/config"
	"github.com/jacobarthurs/gitrisk/internal/git"
	"github.com/jacobarthurs/gitrisk/internal/output"
	"github.com/spf13/cobra"
)

var couplingCmd = &cobra.Command{
	Use:   "coupling <file>",
	Short: "Show all historically coupled files for one file",
	Args:  cobra.ExactArgs(1),
	RunE:  runCoupling,
}

func runCoupling(cmd *cobra.Command, args []string) error {
	file := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	pairs, err := git.CoupledFiles(file, cfg)
	if err != nil {
		return fmt.Errorf("computing coupling: %w", err)
	}

	output.RenderCoupling(file, pairs)
	return nil
}
