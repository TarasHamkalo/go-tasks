package models

import (
	tea "charm.land/bubbletea/v2"
)

type SubModelId int

const (
	ScreenOnboarding SubModelId = iota
	ScreenError      SubModelId = iota
	ScreenPullData   SubModelId = iota
	ScreenActiveChat SubModelId = iota
	TODOScreen SubModelId = iota
)

type Binding struct {
	Key         string
	Description string
}

type SubModel interface {
	Id() SubModelId

	// Main content area only.
	ContentView(width, height int) tea.View

	// Footer key hints.
	ShortHelp() []Binding

	tea.Model
}

type TodoSubModel struct {
}

func (t TodoSubModel) Id() SubModelId { return TODOScreen }

func (t TodoSubModel) Init() tea.Cmd { return nil }

func (t TodoSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return t, nil
}

func (t TodoSubModel) View() tea.View {
	return t.ContentView(500, 500)
}

func (t TodoSubModel) ContentView(width, height int) tea.View {
	return tea.NewView("Todo screen\n")
}

func (t TodoSubModel) ShortHelp() []Binding {
	return []Binding{}
}
