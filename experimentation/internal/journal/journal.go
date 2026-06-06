// Package journal records the status, duration and details of every experiment
// test to an append-only JSONL log and a human-readable Markdown status file.
// This satisfies the rigor requirement: every test is journaled, nothing is
// skipped, and durations + details are always captured.
package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Status of a single test entry.
type Status string

const (
	StatusPass    Status = "PASS"
	StatusFail    Status = "FAIL"
	StatusRunning Status = "RUNNING"
)

// Entry is one journaled test result.
type Entry struct {
	Name       string                 `json:"name"`
	Group      string                 `json:"group"` // e.g. RQ1, RQ2, setup
	Status     Status                 `json:"status"`
	StartedAt  time.Time              `json:"startedAt"`
	DurationMS int64                  `json:"durationMs"`
	Error      string                 `json:"error,omitempty"`
	Details    map[string]any         `json:"details,omitempty"`
}

// Journal writes entries to JSONL + Markdown.
type Journal struct {
	mu      sync.Mutex
	dir     string
	jsonlF  *os.File
	mdPath  string
	entries []Entry
	started time.Time
}

// New opens (creates) a journal in dir. The Markdown file is rewritten on every
// entry so it always reflects the full, current state.
func New(dir string) (*Journal, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "journal.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	j := &Journal{dir: dir, jsonlF: f, mdPath: filepath.Join(dir, "TEST_STATUS.md"), started: time.Now()}
	return j, j.flushMarkdown()
}

// Handle tracks a single in-progress test.
type Handle struct {
	j     *Journal
	name  string
	group string
	start time.Time
}

// Start begins timing a test and records it as RUNNING.
func (j *Journal) Start(group, name string) *Handle {
	return &Handle{j: j, name: name, group: group, start: time.Now()}
}

// Pass records a successful test with details.
func (h *Handle) Pass(details map[string]any) {
	h.j.record(Entry{
		Name: h.name, Group: h.group, Status: StatusPass,
		StartedAt: h.start, DurationMS: time.Since(h.start).Milliseconds(), Details: details,
	})
}

// Fail records a failed test. The experiment treats any FAIL as fatal (no skips,
// no missing results), but recording happens first for full traceability.
func (h *Handle) Fail(err error, details map[string]any) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	h.j.record(Entry{
		Name: h.name, Group: h.group, Status: StatusFail,
		StartedAt: h.start, DurationMS: time.Since(h.start).Milliseconds(), Error: msg, Details: details,
	})
}

// Run times fn, journaling PASS/FAIL automatically. Returns fn's error.
func (j *Journal) Run(group, name string, fn func() (map[string]any, error)) error {
	h := j.Start(group, name)
	details, err := fn()
	if err != nil {
		h.Fail(err, details)
		return err
	}
	h.Pass(details)
	return nil
}

func (j *Journal) record(e Entry) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = append(j.entries, e)
	if line, err := json.Marshal(e); err == nil {
		fmt.Fprintln(j.jsonlF, string(line))
		_ = j.jsonlF.Sync()
	}
	_ = j.flushMarkdownLocked()
	// Console echo for live visibility.
	icon := "x"
	if e.Status == StatusPass {
		icon = "ok"
	}
	fmt.Printf("[%2s] %-5s %-6s %-44s %6dms\n", icon, e.Status, e.Group, e.Name, e.DurationMS)
}

// Summary returns counts of pass/fail.
func (j *Journal) Summary() (pass, fail int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, e := range j.entries {
		switch e.Status {
		case StatusPass:
			pass++
		case StatusFail:
			fail++
		}
	}
	return pass, fail
}

func (j *Journal) flushMarkdown() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.flushMarkdownLocked()
}

func (j *Journal) flushMarkdownLocked() error {
	pass, fail := 0, 0
	for _, e := range j.entries {
		if e.Status == StatusPass {
			pass++
		} else if e.Status == StatusFail {
			fail++
		}
	}
	var b []byte
	b = append(b, []byte("# Experiment test status\n\n")...)
	b = append(b, []byte(fmt.Sprintf("- Run started: %s\n", j.started.UTC().Format(time.RFC3339)))...)
	b = append(b, []byte(fmt.Sprintf("- Last update: %s\n", time.Now().UTC().Format(time.RFC3339)))...)
	b = append(b, []byte(fmt.Sprintf("- Totals: **%d PASS**, **%d FAIL**, %d total\n\n", pass, fail, len(j.entries)))...)
	b = append(b, []byte("| # | Group | Test | Status | Duration | Details |\n")...)
	b = append(b, []byte("|--:|-------|------|:------:|---------:|---------|\n")...)
	for i, e := range j.entries {
		det := ""
		if len(e.Details) > 0 {
			if d, err := json.Marshal(e.Details); err == nil {
				det = string(d)
			}
		}
		if e.Error != "" {
			det = "ERROR: " + e.Error + " " + det
		}
		b = append(b, []byte(fmt.Sprintf("| %d | %s | %s | %s | %dms | %s |\n",
			i+1, e.Group, e.Name, e.Status, e.DurationMS, det))...)
	}
	return os.WriteFile(j.mdPath, b, 0o644)
}

// Close flushes and closes the journal.
func (j *Journal) Close() error {
	_ = j.flushMarkdown()
	return j.jsonlF.Close()
}
