package git

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jacobarthurs/gitrisk/internal/config"
)

func DiffFiles(base, head string) ([]string, error) {
	out, err := run("git", "diff", "--name-only", base+"..."+head)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

func StagedFiles() ([]string, error) {
	out, err := run("git", "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

func AllTouchedFiles(cfg *config.Config) ([]string, error) {
	after := fmt.Sprintf("--after=%d days ago", cfg.LookbackDays)
	out, err := run("git", "log", "--no-merges", after, "--name-only", "--format=")
	if err != nil {
		return nil, err
	}
	return uniqueSlice(splitLines(out)), nil
}

func normalizePath(p string) string {
	p = strings.TrimPrefix(p, `.\`)
	p = strings.TrimPrefix(p, `./`)
	return filepath.ToSlash(p)
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line != "" {
			out = append(out, normalizePath(line))
		}
	}
	return out
}

func uniqueSlice(lines []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, l := range lines {
		if _, ok := seen[l]; !ok {
			seen[l] = struct{}{}
			out = append(out, l)
		}
	}
	return out
}
