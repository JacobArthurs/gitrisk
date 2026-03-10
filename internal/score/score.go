package score

import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/jacobarthurs/gitrisk/internal/config"
	"github.com/jacobarthurs/gitrisk/internal/git"
)

type Level int

const (
	Low Level = iota
	Medium
	High
	Critical
)

func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "medium":
		return Medium
	case "high":
		return High
	case "critical":
		return Critical
	default:
		return Low
	}
}

func LevelLabel(l Level) string {
	switch l {
	case Medium:
		return "MED"
	case High:
		return "HIGH"
	case Critical:
		return "CRIT"
	default:
		return "LOW"
	}
}

func (l Level) MarshalJSON() ([]byte, error) {
	return json.Marshal(LevelLabel(l))
}

type FileResult struct {
	File          string  `json:"file"`
	ChurnScore    float64 `json:"churn_score"`
	AuthorScore   float64 `json:"author_score"`
	BugfixScore   float64 `json:"bugfix_score"`
	CouplingScore float64 `json:"coupling_score,omitempty"`
	Composite     float64 `json:"composite"`
	Level         Level   `json:"level"`
	// Raw values for explain output — omitted from JSON
	ChurnRaw  int `json:"-"`
	AuthorRaw int `json:"-"`
	BugfixRaw int `json:"-"`
}

func ScoreAll(signals []git.FileSignals, norms *git.Norms, cfg *config.Config, withCoupling bool) []FileResult {
	results := make([]FileResult, 0, len(signals))
	for _, sig := range signals {
		results = append(results, scoreOne(sig, norms, cfg, withCoupling))
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Composite > results[j].Composite
	})
	return results
}

func scoreOne(sig git.FileSignals, norms *git.Norms, cfg *config.Config, withCoupling bool) FileResult {
	churnScore := normalize(float64(sig.ChurnCount), float64(norms.MaxChurn), float64(cfg.FloorChurn))
	authorScore := normalize(float64(sig.AuthorCount), float64(norms.MaxAuthors), float64(cfg.FloorFragmentation))
	bugfixScore := normalize(float64(sig.BugfixCount), float64(norms.MaxBugfixes), float64(cfg.FloorBugfix))
	couplingScore := sig.CouplingScore

	var composite float64

	if w := cfg.Weights; withCoupling {
		composite = churnScore*w.Churn + authorScore*w.Fragmentation +
			bugfixScore*w.Bugfix +
			couplingScore*w.Coupling
	} else {
		// Redistribute coupling weight proportionally across remaining three signals
		total := w.Churn + w.Fragmentation + w.Bugfix
		composite = churnScore*(w.Churn/total) +
			authorScore*(w.Fragmentation/total) +
			bugfixScore*(w.Bugfix/total)
	}

	composite = math.Round(composite*100) / 100

	return FileResult{
		File:          sig.File,
		ChurnScore:    round(churnScore),
		AuthorScore:   round(authorScore),
		BugfixScore:   round(bugfixScore),
		CouplingScore: round(couplingScore),
		Composite:     composite,
		Level:         levelFromScore(composite, cfg),
		ChurnRaw:      sig.ChurnCount,
		AuthorRaw:     sig.AuthorCount,
		BugfixRaw:     sig.BugfixCount,
	}
}

func normalize(val, max, floor float64) float64 {
	denom := math.Max(max, floor)
	if denom == 0 {
		return 0
	}
	return math.Min(val/denom*10, 10)
}

func round(v float64) float64 {
	return math.Round(v*100) / 100
}

func levelFromScore(s float64, cfg *config.Config) Level {
	switch {
	case s >= cfg.Thresholds.Critical:
		return Critical
	case s >= cfg.Thresholds.High:
		return High
	case s >= cfg.Thresholds.Medium:
		return Medium
	default:
		return Low
	}
}

func FilterByLevel(results []FileResult, min Level) []FileResult {
	var out []FileResult
	for _, r := range results {
		if r.Level >= min {
			out = append(out, r)
		}
	}
	return out
}

func AnyMeetsLevel(results []FileResult, min Level) bool {
	for _, r := range results {
		if r.Level >= min {
			return true
		}
	}
	return false
}

func TopN(results []FileResult, n int) []FileResult {
	if len(results) <= n {
		return results
	}
	return results[:n]
}
