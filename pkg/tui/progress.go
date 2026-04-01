package tui

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"
)

// Progress is a lightweight TTY-aware progress counter. It writes to stderr
// so that stdout remains clean for piped usage. When stdout is not a TTY,
// it becomes a no-op.
type Progress struct {
	label     string
	total     int
	completed int
	isTTY     bool
	mu        sync.Mutex
}

// NewProgress creates a new Progress tracker. label is the prefix shown on the
// progress line. total is the expected number of Increment calls.
func NewProgress(label string, total int) *Progress {
	return &Progress{
		label: label,
		total: total,
		isTTY: term.IsTerminal(int(os.Stderr.Fd())),
	}
}

// Increment records one unit of completed work and refreshes the progress line.
func (p *Progress) Increment() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completed++
	if !p.isTTY {
		return
	}
	if p.completed < p.total {
		fmt.Fprintf(os.Stderr, "\r%s [%d/%d]", p.label, p.completed, p.total)
	} else {
		// Clear the progress line when all work is done.
		fmt.Fprintf(os.Stderr, "\r\033[K")
	}
}
