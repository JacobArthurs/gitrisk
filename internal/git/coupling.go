package git

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jacobarthurs/gitrisk/internal/config"
)

type CouplingPair struct {
	File      string
	CoFile    string
	Frequency float64 // 0.0–1.0, relative to File's commit count
}

type CouplingWarning struct {
	File    string  `json:"file"`
	Missing string  `json:"missing"`
	Freq    float64 `json:"frequency"`
}

func CouplingWarnings(changedFiles []string, cfg *config.Config) ([]CouplingWarning, error) {
	matrix, err := buildCouplingMatrix(cfg)
	if err != nil {
		return nil, err
	}

	inDiff := map[string]struct{}{}
	for _, f := range changedFiles {
		inDiff[f] = struct{}{}
	}

	var warnings []CouplingWarning
	for _, file := range changedFiles {
		for co, freq := range matrix[file] {
			if freq >= cfg.CouplingThreshold {
				if _, present := inDiff[co]; !present {
					warnings = append(warnings, CouplingWarning{
						File:    file,
						Missing: co,
						Freq:    freq,
					})
				}
			}
		}
	}
	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].File != warnings[j].File {
			return warnings[i].File < warnings[j].File
		}
		return warnings[i].Freq > warnings[j].Freq
	})
	return warnings, nil
}

func CoupledFiles(file string, cfg *config.Config) ([]CouplingPair, error) {
	matrix, err := buildCouplingMatrix(cfg)
	if err != nil {
		return nil, err
	}

	file = normalizePath(file)
	var pairs []CouplingPair
	for co, freq := range matrix[file] {
		if freq >= cfg.CouplingThreshold {
			pairs = append(pairs, CouplingPair{File: file, CoFile: co, Frequency: freq})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Frequency > pairs[j].Frequency
	})
	return pairs, nil
}

func isCommitHash(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func buildCouplingMatrix(cfg *config.Config) (map[string]map[string]float64, error) {
	after := fmt.Sprintf("--after=%d days ago", cfg.CouplingLookbackDays)

	out, err := run("git", "log", "--no-merges", after, "--name-only", "--format=%H")
	if err != nil {
		return nil, err
	}

	var commits [][]string
	var current []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		switch {
		case isCommitHash(line):
			if len(current) > 0 {
				commits = append(commits, current)
				current = nil
			}
		case line != "":
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		commits = append(commits, current)
	}

	fileCounts := map[string]int{}
	coOccur := map[string]map[string]int{}

	for _, files := range commits {
		for _, f := range files {
			fileCounts[f]++
		}
		for i, a := range files {
			for _, b := range files[i+1:] {
				if coOccur[a] == nil {
					coOccur[a] = map[string]int{}
				}
				if coOccur[b] == nil {
					coOccur[b] = map[string]int{}
				}
				coOccur[a][b]++
				coOccur[b][a]++
			}
		}
	}

	matrix := map[string]map[string]float64{}
	for file, coMap := range coOccur {
		matrix[file] = map[string]float64{}
		for co, count := range coMap {
			if fileCounts[file] > 0 {
				matrix[file][co] = float64(count) / float64(fileCounts[file])
			}
		}
	}
	return matrix, nil
}
