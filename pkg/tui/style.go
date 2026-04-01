package tui

import "github.com/fatih/color"

// Color styles used across all render functions.
var (
	HeaderStyle  = color.New(color.Bold, color.FgHiWhite)
	ErrorStyle   = color.New(color.FgRed)
	SuccessStyle = color.New(color.FgGreen)
	WarnStyle    = color.New(color.FgYellow)
	InfoStyle    = color.New(color.FgCyan)
)
