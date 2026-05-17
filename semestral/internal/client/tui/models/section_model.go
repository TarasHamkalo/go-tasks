package models

import (
	"gomessenger/internal/client/tui"

	tea "charm.land/bubbletea/v2"
)

type SectionModel interface {
	SetEngaged(engaged bool) 

	// Main content area only.
	ContentView(width int, height int) string

	// Footer key hints.
	ShortHelp() []tui.Binding

	tea.Model
}

