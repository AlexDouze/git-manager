package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// The line renderers (status/prune/update) write to output, downsampling ANSI
// colors to whatever the destination supports via a colorprofile.Writer. Tests
// swap output to capture text; --no-color forces the ASCII profile so styled
// strings come out plain.
var (
	out        io.Writer = os.Stdout
	forceASCII bool
)

// SetOutput redirects where the line renderers write. Primarily for tests.
func SetOutput(w io.Writer) { out = w }

// SetNoColor forces color off (true) or restores auto-detection (false) for the
// line renderers. Wired to the global --no-color flag.
func SetNoColor(disable bool) { forceASCII = disable }

// lineStyle is a Lip Gloss style paired with fmt-style print helpers that mirror
// the fatih/color API the renderers were written against. Output is passed
// through a colorprofile.Writer so colors degrade gracefully (and vanish under
// --no-color or when writing to a pipe).
type lineStyle struct {
	style lipgloss.Style
}

func newLineStyle(s lipgloss.Style) *lineStyle { return &lineStyle{style: s} }

// writer builds a profile-aware writer over the current output. When --no-color
// is set the ASCII profile is used, which strips all styling.
func writer() *colorprofile.Writer {
	w := colorprofile.NewWriter(out, os.Environ())
	if forceASCII {
		w.Profile = colorprofile.Ascii
	}
	return w
}

// render styles s without letting Lip Gloss pad multi-line blocks to a uniform
// width: any trailing newline is stripped before Render and re-appended after,
// so styled output stays as tight as the fatih/color version it replaced.
func (l *lineStyle) render(s string) string {
	trimmed := strings.TrimRight(s, "\n")
	trailing := s[len(trimmed):]
	return l.style.Render(trimmed) + trailing
}

func (l *lineStyle) Printf(format string, a ...any) {
	fmt.Fprint(writer(), l.render(fmt.Sprintf(format, a...)))
}

func (l *lineStyle) Println(a ...any) {
	fmt.Fprintln(writer(), l.render(fmt.Sprint(a...)))
}

// Color styles used across all render functions. Colors match the previous
// fatih/color palette (bold hi-white header, red/green/yellow/cyan states).
var (
	HeaderStyle  = newLineStyle(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")))
	ErrorStyle   = newLineStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("9")))
	SuccessStyle = newLineStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("10")))
	WarnStyle    = newLineStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("11")))
	InfoStyle    = newLineStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("14")))
)
