package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type mainModel struct {
	activeModel     Decor
	models          []Decor
	currentModelIdx int
}

func (m mainModel) InitialModel() mainModel {

	m = mainModel{
		currentModelIdx: 0,
		models:          []Decor{LanguageModel{}.InitialModel()},
	}

	m.activeModel = m.models[0]

	return m
}

func (m mainModel) Init() tea.Cmd {
	return nil
}

func (m mainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "n":
			if m.currentModelIdx+1 > len(m.models)-1 {
				m.activeModel = m.models[0]
				break
			}
			m.activeModel = m.models[m.currentModelIdx+1]
		}
	}

	m.activeModel, cmd = m.activeModel.Update(msg)
	return m, cmd
}

func (m mainModel) View() string {
	return fmt.Sprintf(
		"%s\n\nPress q to quit. Press c to continue.",
		m.activeModel.View(),
	)
}

func main() {
	mainModel := mainModel{}.InitialModel()
	p := tea.NewProgram(mainModel)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
