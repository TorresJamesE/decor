package models

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Decor struct {
	tea.Model
	choices  []string
	cursor   int
	Selected map[int]struct{}
}
