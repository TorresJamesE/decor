package models

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type LanguageModel struct {
	Decor
}

func (m Decor) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m Decor) Selections() []string {
	var selectedLanguages []string
	for index := range m.Selected {
		selectedLanguages = append(selectedLanguages, m.choices[index])
	}
	return selectedLanguages
}

func (m Decor) InitialModel() Decor {
	return Decor{
		choices:  []string{"Go", "Python", "Rust", "C++", "Java"},
		Selected: make(map[int]struct{}),
	}
}

func (m Decor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "n":
			// This is where we would transition to the next model, passing the selected languages.
			selectedLanguages := m.Selections()
			fmt.Printf("Selected languages: %s\n", strings.Join(selectedLanguages, ", "))
			return NewDownloadInstallModel(m.Selections()), nil // This is just a placeholder. You would return the new model here.

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			_, ok := m.Selected[m.cursor]
			if ok {
				delete(m.Selected, m.cursor)
			} else {
				m.Selected[m.cursor] = struct{}{}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m Decor) View() string {
	// The header
	var s strings.Builder
	s.WriteString("What programming language(s) do you want to install?\n\n")

	// Iterate over our choices
	for i, choice := range m.choices {

		// Is the cursor pointing at this choice?
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">" // cursor!
		}

		// Is this choice selected?
		checked := " " // not selected
		if _, ok := m.Selected[i]; ok {
			checked = "x" // selected!
		}

		// Render the row
		fmt.Fprintf(&s, "%s [%s] %s\n", cursor, checked, choice)
	}

	// Send the UI for rendering
	fmt.Fprintln(&s, "\nPress space or enter to select.\nPress up/down or k/j to navigate. \nPress n to continue. \nPress q or ctrl+c to quit.")
	return s.String()
}
