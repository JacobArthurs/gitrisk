package git

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/jacobarthurs/gitrisk/internal/config"
)

type FileSignals struct {
	File          string
	ChurnCount    int
	AuthorCount   int
	BugfixCount   int
	CouplingScore float64 // pre-computed only when --coupling is active
}

type Norms struct {
	MaxChurn    int
	MaxAuthors  int
	MaxBugfixes int
}

var bugfixRe = regexp.MustCompile(`(?i)\b(fix|bug|hotfix|patch|defect)\b`)

func CollectSignals(files []string, cfg *config.Config) ([]FileSignals, error) {
	results := make([]FileSignals, len(files))
	errs := make([]error, len(files))

	sem := make(chan struct{}, cfg.Workers)
	var wg sync.WaitGroup

	for i, file := range files {
		wg.Add(1)
		go func(idx int, f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sig, err := signalsForFile(f, cfg)
			results[idx] = sig
			errs[idx] = err
		}(i, file)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func signalsForFile(file string, cfg *config.Config) (FileSignals, error) {
	after := fmt.Sprintf("--after=%d days ago", cfg.LookbackDays)

	churnOut, err := run("git", "log", "--no-merges", after, "--oneline", "--", file)
	if err != nil {
		return FileSignals{}, err
	}

	authOut, err := run("git", "log", "--no-merges", after, "--format=%ae", "--", file)
	if err != nil {
		return FileSignals{}, err
	}

	// Use subject lines + regex instead of --grep -P, which is unavailable on some git distributions.
	subjOut, err := run("git", "log", "--no-merges", after, "--format=%s", "--", file)
	if err != nil {
		return FileSignals{}, err
	}

	bugfixes := 0
	for _, line := range strings.Split(strings.TrimSpace(subjOut), "\n") {
		if line != "" && bugfixRe.MatchString(line) {
			bugfixes++
		}
	}

	return FileSignals{
		File:        file,
		ChurnCount:  countLines(churnOut),
		AuthorCount: uniqueEmails(authOut),
		BugfixCount: bugfixes,
	}, nil
}

func ComputeNorms(cfg *config.Config) (*Norms, error) {
	after := fmt.Sprintf("--after=%d days ago", cfg.LookbackDays)

	out, err := run("git", "log", "--no-merges", after, "--name-only", "--format=")
	if err != nil {
		return nil, err
	}
	allFiles := uniqueSlice(splitLines(out))

	signals, err := CollectSignals(allFiles, cfg)
	if err != nil {
		return nil, err
	}

	norms := &Norms{}
	for _, s := range signals {
		if s.ChurnCount > norms.MaxChurn {
			norms.MaxChurn = s.ChurnCount
		}
		if s.AuthorCount > norms.MaxAuthors {
			norms.MaxAuthors = s.AuthorCount
		}
		if s.BugfixCount > norms.MaxBugfixes {
			norms.MaxBugfixes = s.BugfixCount
		}
	}
	return norms, nil
}

func countLines(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

func uniqueEmails(s string) int {
	seen := map[string]struct{}{}
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if line != "" {
			seen[line] = struct{}{}
		}
	}
	return len(seen)
}
