package main

import (
	"fmt"
	"os"

	"decor/models"

	tea "github.com/charmbracelet/bubbletea"
)

type MainModel struct {
	activeModel     tea.Model
	models          []tea.Model
	currentModelIdx int
}

func (m MainModel) InitialModel() MainModel {

	m = MainModel{
		currentModelIdx: 0,
		models:          []tea.Model{models.LanguageModel{}.InitialModel()},
	}

	m.activeModel = m.models[0]
	return m
}

func (m MainModel) Init() tea.Cmd {
	return m.activeModel.Init()
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "n":
			if m.currentModelIdx+1 > len(m.models)-1 {
				newModel := models.NewDownloadInstallModel(m.activeModel.(models.Decor).Selections())
				m.models = append(m.models, newModel)
				m.activeModel = newModel
				m.currentModelIdx = len(m.models) - 1
				return m, newModel.Init()
			}
			m.currentModelIdx++
			m.activeModel = m.models[m.currentModelIdx]
		}
	}

	updatedModel, cmd := m.activeModel.Update(msg)
	m.activeModel = updatedModel
	m.models[m.currentModelIdx] = updatedModel
	return m, cmd
}

func (m MainModel) View() string {
	return m.activeModel.View()
}

func main() {
	fmt.Printf("Welcome to Decor! This tool will help you install ('decorate') your environment with what you need.\n\n")

	MainModel := MainModel{}.InitialModel()
	p := tea.NewProgram(MainModel)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
