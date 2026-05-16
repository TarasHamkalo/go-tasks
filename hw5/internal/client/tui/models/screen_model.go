package models

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

type SubModelId int

const (
	ScreenOnboarding SubModelId = iota
	TODOScreen       SubModelId = iota
)

type SubModel interface {
	Id() SubModelId

	// Main content area only.
	ContentView(width, height int) tea.View

	// Footer key hints.
	ShortHelp() map[string]string

	tea.Model
}

type TodoSubModel struct {
}

func (t TodoSubModel) Id() SubModelId {
	return TODOScreen
}

func (t TodoSubModel) Init() tea.Cmd {
	return nil 
}
func (t TodoSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	fmt.Println("Todo reached")
	return t, tea.Quit
}

func (t TodoSubModel) View() (tea.View) {
	return tea.NewView("Todo screen\n")
}


func (t TodoSubModel) ContentView(width, height int) tea.View {
	return tea.NewView("Todo screen\n")
}

func (t TodoSubModel) ShortHelp() map[string]string {
	return map[string]string{}
}

