package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jacobarthurs/gitrisk/internal/git"
	"github.com/jacobarthurs/gitrisk/internal/score"
)

type Renderer struct {
	format string
}

func NewRenderer(format string) *Renderer {
	return &Renderer{format: format}
}

func (r *Renderer) Render(results []score.FileResult, warnings []git.CouplingWarning) error {
	switch r.format {
	case "json":
		return renderJSON(results, warnings)
	default:
		return renderTable(results, warnings)
	}
}

var (
	styleCrit   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	styleHigh   = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	styleMed    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	styleLow    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
)

func colorLevel(label string) string {
	switch label {
	case "CRIT":
		return styleCrit.Render(label)
	case "HIGH":
		return styleHigh.Render(label)
	case "MED":
		return styleMed.Render(label)
	default:
		return styleLow.Render(label)
	}
}

func renderTable(results []score.FileResult, warnings []git.CouplingWarning) error {
	if len(results) == 0 {
		fmt.Println("No files meet the risk threshold.")
		return nil
	}

	maxFile := len("File")
	for _, r := range results {
		if len(r.File) > maxFile {
			maxFile = len(r.File)
		}
	}
	width := maxFile + 20

	fmt.Println()
	fmt.Println(styleHeader.Render("gitrisk — PR Risk Analysis"))
	fmt.Println(strings.Repeat("─", width))
	// Use manual padding instead of %-Ns so ANSI escape codes don't skew column widths.
	fmt.Printf("%s%s  %s%s  %s\n",
		styleHeader.Render("File"), strings.Repeat(" ", maxFile-len("File")),
		styleHeader.Render("Risk"), strings.Repeat(" ", max(0, 6-len("Risk"))),
		styleHeader.Render("Score"),
	)
	fmt.Println(strings.Repeat("─", width))

	for _, r := range results {
		label := score.LevelLabel(r.Level)
		fmt.Printf("%-*s  %s%s  %.1f\n",
			maxFile, r.File,
			colorLevel(label), strings.Repeat(" ", max(0, 6-len(label))),
			r.Composite,
		)
	}
	fmt.Println(strings.Repeat("─", width))
	fmt.Println()

	if len(warnings) > 0 {
		fmt.Println(styleWarn.Render("⚠  Coupling warnings:"))
		for _, w := range warnings {
			fmt.Printf("  %s co-changes with %s (%.0f%% of commits) — not in this diff\n",
				w.File, w.Missing, w.Freq*100)
		}
		fmt.Println()
	}

	return nil
}

func renderJSON(results []score.FileResult, warnings []git.CouplingWarning) error {
	payload := struct {
		Results  []score.FileResult    `json:"results"`
		Warnings []git.CouplingWarning `json:"coupling_warnings,omitempty"`
	}{
		Results:  results,
		Warnings: warnings,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func RenderExplain(r score.FileResult, norms *git.Norms, lookbackDays int) {
	fmt.Println()
	fmt.Println(styleHeader.Render(r.File))
	fmt.Println(strings.Repeat("─", len(r.File)+4))
	fmt.Printf("Churn     (last %dd):  %3d commits  → score %.1f  (repo max: %d)\n",
		lookbackDays, r.ChurnRaw, r.ChurnScore, norms.MaxChurn)
	fmt.Printf("Authors   (last %dd):  %3d authors  → score %.1f  (repo max: %d)\n",
		lookbackDays, r.AuthorRaw, r.AuthorScore, norms.MaxAuthors)
	fmt.Printf("Bug-fixes (last %dd):  %3d commits  → score %.1f  (repo max: %d)\n",
		lookbackDays, r.BugfixRaw, r.BugfixScore, norms.MaxBugfixes)
	fmt.Println()
	fmt.Printf("Composite risk: %.1f / 10  [%s]\n",
		r.Composite, colorLevel(score.LevelLabel(r.Level)))
	fmt.Println()
}

func RenderCoupling(file string, pairs []git.CouplingPair) {
	if len(pairs) == 0 {
		fmt.Printf("No coupling relationships above threshold found for %s.\n", file)
		return
	}
	fmt.Println()
	fmt.Println(styleHeader.Render("Coupling: " + file))
	fmt.Println(strings.Repeat("─", 52))
	for _, p := range pairs {
		fmt.Printf("  %-44s  %.0f%%\n", p.CoFile, p.Frequency*100)
	}
	fmt.Println()
}
