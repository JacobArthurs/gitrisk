# gitrisk (Work in progress)

## What it is

A CLI binary that analyzes the files changed in your current branch or staged diff and outputs a risk score for each file, ranked by danger. It answers the question a reviewer should ask before opening a PR: **"which of these changes are actually risky?"**

Uses only local git history. No network, no account, no external service. Runs on private repos with no data leaving the machine. Integrates into any CI pipeline in under 30 seconds.

---

## The core insight

A one-line change to a file that has been modified 40 times in the last 3 months by 8 different authors, and has 5 bug-fix commits touching it, is more dangerous than a 500-line change to a test file nobody has touched in a year. Lines changed is a bad proxy for risk. Git history is a far better one.

This is the idea Adam Tornhill wrote an entire book about (*Your Code as a Crime Scene*), that CodeScene built a company on, and that no free local CLI tool implements at PR time.

---

## The four signals

Every file in the diff gets scored across four independent dimensions:

**1. Churn rate**
How frequently has this file changed in the last 90 days. High churn = unstable code, unclear ownership, or a load-bearing file everyone touches constantly. Churn = commit count, computed from `git log --no-merges --oneline -- <file>`.

**2. Ownership fragmentation**
How many distinct authors have touched this file in the last 90 days. One author = clear ownership. Eight authors = coordination risk, inconsistent patterns, nobody fully understands it. Computed from `git log --no-merges --format="%ae"`. Author identity uses email deduplication; acknowledged limitation: engineers with multiple emails or inconsistent `user.email` configs will be over-counted. Document this rather than silently treating it as reliable.

**3. Bug-fix proximity**
How many commits touching this file in the last 90 days contained bug-fix language in the message. A file that required 6 bug fixes recently is structurally troubled.

Fetched via `git log --no-merges --after="90 days ago" --format="%s" -- <file>` (subject line only), then filtered Go-side using `regexp.MustCompile(`(?i)\b(fix|bug|hotfix|patch|defect)\b`)`. This avoids using `--grep` with `-P` (PCRE), which is not available on all git versions. Go-side filtering is fully portable and gives complete control over the pattern.

**4. Change coupling**
Files that historically change together but are absent from this diff. If files A and B have co-changed in 80% of commits over the past 6 months, and your PR touches A but not B, that's a warning that you may have forgotten something. Computed from co-occurrence frequency across `git log --no-merges`. Coupling is opt-in via `--coupling` flag due to performance cost on large repos (see Technical Implementation).

---

## Scoring and normalization

Each signal is scored 0–10 using **repo-relative normalization**: the highest observed value across all files in the repo's recent history anchors the top of the scale. This ensures scores are meaningful on both low-activity and high-activity repos rather than against arbitrary absolute ceilings.

Normalization uses a floor on the denominator to prevent misleading scores on new or sparse repos:

```text
score = (raw_value / max(signal_max, minimum_floor)) * 10
```

Default floors: churn = 10 commits, fragmentation = 3 authors, bugfix = 3 commits. These are configurable. Without a floor, a repo with a single file at 2 churn commits would score that file 10.0, which is meaningless.

```text
risk = (churn × 0.30) + (fragmentation × 0.25) + (bugfix × 0.30) + (coupling × 0.15)
```

When `--coupling` is not enabled, coupling's 0.15 weight is redistributed proportionally across the other three signals so the composite still sums to 10:

```text
churn × 0.353 + fragmentation × 0.294 + bugfix × 0.353
```

This is computed automatically. The user never sees adjusted weights, but the score is not artificially capped at 8.5.

Weights are configurable. Output buckets:

| Score | Bucket   |
|-------|----------|
| 0–3.9 | LOW      |
| 4–6.9 | MEDIUM   |
| 7–8.9 | HIGH     |
| 9–10  | CRITICAL |

Change coupling outputs separately as a warning, not a file score — "you changed X, historically Y also changes with it, Y is not in this diff."

---

## How it is used

**Basic usage — run on current branch before opening PR:**

```bash
gitrisk analyze
```

Diffs current branch against main, scores every changed file, outputs ranked risk table.

**Specific branch comparison:**

```bash
gitrisk analyze --base main --head feature/payment-refactor
```

**Staged changes only:**

```bash
gitrisk analyze --staged
```

**Include coupling analysis:**

```bash
gitrisk analyze --coupling
```

**CI integration — fail on high risk:**

```bash
gitrisk analyze --base main --format json
gitrisk analyze --base main --threshold high --exit-code
```

Returns exit code 1 if any file scores HIGH or above. Plugs into GitHub Actions, Jenkins, any CI pipeline.

**Output:**

```text
┌─────────────────────────────────────────────────────────┐
│ gitrisk — PR Risk Analysis                              │
├─────────────────────────────┬───────┬───────────────────┤
│ File                        │ Risk  │ Signals           │
├─────────────────────────────┼───────┼───────────────────┤
│ src/payments/processor.go   │ HIGH  │ churn             │
│ src/auth/jwt.go             │ HIGH  │ bugfix            │
│ internal/db/migrations.go   │ MED   │ authors           │
│ cmd/cli/flags.go            │ LOW   │ —                 │
│ README.md                   │ LOW   │ —                 │
└─────────────────────────────┴───────┴───────────────────┘

⚠ Coupling warning:
  src/payments/processor.go historically co-changes with
  src/payments/validator.go (87% of commits), and is not in this diff
```

---

## Configuration file

`.gitrisk.yml` at repo root:

```yaml
lookback_days: 90           # history window for churn, fragmentation, bugfix signals
coupling_lookback_days: 180 # history window for coupling (longer = more signal)
coupling_threshold: 0.75    # co-change frequency to trigger warning
weights:
  churn: 0.30
  fragmentation: 0.25
  bugfix: 0.30
  coupling: 0.15
ignore:
  - "**/*_test.go"
  - "docs/**"
  - "*.md"
thresholds:
  high: 7.0
  critical: 9.0
```

Zero config required, all defaults are sensible out of the box.

---

## CLI surface

```bash
gitrisk analyze                     run analysis on current branch vs main
gitrisk analyze --staged            run analysis on staged changes only
gitrisk analyze --coupling          include change coupling analysis (slower)
gitrisk analyze --base <ref>        set base ref for diff (default: main)
gitrisk analyze --head <ref>        set head ref for diff (default: current branch)
gitrisk analyze --format json       output as JSON instead of table
gitrisk analyze --threshold <level> set minimum risk level to report (low/medium/high/critical)
gitrisk analyze --exit-code         return exit code 1 if any file meets or exceeds threshold
gitrisk explain <file>              show full signal breakdown for one file
gitrisk hotspots                    show top 10 riskiest files repo-wide (full lookback, not PR-scoped)
gitrisk coupling <file>             show all historically coupled files for one file
```

`explain` shows the raw numbers behind the score:

```bash
$ gitrisk explain src/payments/processor.go

src/payments/processor.go
─────────────────────────
Churn (last 90d):        43 commits    → score 8.6  (repo max: 50)
Unique authors (90d):    7 authors     → score 7.0  (repo max: 10)
Bug-fix commits (90d):   4 commits     → score 6.0  (repo max: 7)
Coupled files:           validator.go  → 87% co-change rate

Composite risk: 7.4 / 10  [HIGH]
```

The repo max shown per-signal makes the normalization transparent.

---

## Technical implementation in Go

**Git operations**: Shell out to git directly via `os/exec`. No go-git library — native git is faster, always available, and the commands needed are stable.

**All `git log` commands include `--no-merges`** to prevent merge commits from inflating churn and author counts on merge-heavy workflows.

**Diff detection**: `git diff --name-only <base>...<head>`

**Churn**: `git log --no-merges --after="90 days ago" --oneline -- <file>` — count output lines per file.

**Fragmentation**: `git log --no-merges --after="90 days ago" --format="%ae" -- <file>` — deduplicate, count unique emails.

**Bug-fix proximity**: `git log --no-merges --after="90 days ago" --format="%s" -- <file>` — pipe commit subjects through Go-side regex `(?i)\b(fix|bug|hotfix|patch|defect)\b`, count matches. Avoids `--grep -P` (PCRE) which is unavailable on some git distributions, including macOS Xcode git.

**Normalization**: Before scoring, run churn/fragmentation/bugfix queries across all files touched in the lookback window to establish per-signal maximums. Score each file as `(raw_value / max(signal_max, floor)) * 10`. Default floors: 10 (churn), 3 (fragmentation), 3 (bugfix). Cache this pass — it runs once per invocation, not per file.

**Change coupling**: `git log --no-merges --after="180 days ago" --name-only --format=""` gives all commits with their changed files. Build a co-occurrence matrix in memory. For each file in the diff, find all files with co-change frequency above threshold that are absent from the current diff. This is opt-in via `--coupling` flag.

**Performance**: File-level git commands run concurrently with a goroutine worker pool (configurable, default 8 workers). Target under 3 seconds on repos with 10k commits for the default (no-coupling) path. Coupling on large repos (50k+ commits) will be slower — document expected runtime and suggest capping `coupling_lookback_days` for monorepos.

**Dependencies:**

- `github.com/spf13/cobra` — CLI structure
- `github.com/charmbracelet/lipgloss` — terminal table styling
- `gopkg.in/yaml.v3` — config file
- Everything else is standard library

https://claude.ai/chat/b77fc256-a027-4e29-823f-cd13edc25c57
