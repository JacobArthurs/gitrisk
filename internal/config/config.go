package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Weights struct {
	Churn         float64 `yaml:"churn"`
	Fragmentation float64 `yaml:"fragmentation"`
	Bugfix        float64 `yaml:"bugfix"`
	Coupling      float64 `yaml:"coupling"`
}

type Thresholds struct {
	Medium   float64 `yaml:"medium"`
	High     float64 `yaml:"high"`
	Critical float64 `yaml:"critical"`
}

type Config struct {
	LookbackDays         int        `yaml:"lookback_days"`
	CouplingLookbackDays int        `yaml:"coupling_lookback_days"`
	CouplingThreshold    float64    `yaml:"coupling_threshold"`
	Weights              Weights    `yaml:"weights"`
	Ignore               []string   `yaml:"ignore"`
	Thresholds           Thresholds `yaml:"thresholds"`

	// Runtime-only (not set from YAML)
	FloorChurn         int
	FloorFragmentation int
	FloorBugfix        int
	Workers            int
}

var defaults = Config{
	LookbackDays:         90,
	CouplingLookbackDays: 180,
	CouplingThreshold:    0.75,
	Weights: Weights{
		Churn:         0.30,
		Fragmentation: 0.25,
		Bugfix:        0.30,
		Coupling:      0.15,
	},
	Thresholds: Thresholds{
		Medium:   4.0,
		High:     7.0,
		Critical: 9.0,
	},
	FloorChurn:         10,
	FloorFragmentation: 3,
	FloorBugfix:        3,
	Workers:            8,
}

func gitRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "."
	}
	return strings.TrimSpace(string(out))
}

func Load() (*Config, error) {
	cfg := defaults

	data, err := os.ReadFile(filepath.Join(gitRoot(), ".gitrisk.yml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Restore runtime-only fields
	cfg.FloorChurn = defaults.FloorChurn
	cfg.FloorFragmentation = defaults.FloorFragmentation
	cfg.FloorBugfix = defaults.FloorBugfix
	cfg.Workers = defaults.Workers

	return &cfg, nil
}

func FilterIgnored(files, patterns []string) []string {
	if len(patterns) == 0 {
		return files
	}
	var out []string
	for _, f := range files {
		ignored := false
		for _, pattern := range patterns {
			if matchGlob(pattern, f) {
				ignored = true
				break
			}
		}
		if !ignored {
			out = append(out, f)
		}
	}
	return out
}

func matchGlob(pattern, path string) bool {
	pattern = filepath.ToSlash(strings.TrimSuffix(pattern, "/"))
	path = filepath.ToSlash(path)

	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}

	if !strings.Contains(pattern, "/") {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}

	parts := strings.Split(path, "/")
	for i := 1; i < len(parts); i++ {
		if matched, _ := filepath.Match(pattern, strings.Join(parts[:i], "/")); matched {
			return true
		}
	}
	return false
}
