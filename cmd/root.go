/*
Copyright © 2026 JACOB ARTHURS
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:          "gitrisk",
	Short:        "PR risk analysis from local git history",
	Long:         `gitrisk scores files changed in your branch by historical risk signals: churn, ownership fragmentation, bug-fix proximity, and change coupling.`,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(explainCmd)
	rootCmd.AddCommand(hotspotsCmd)
	rootCmd.AddCommand(couplingCmd)
}
