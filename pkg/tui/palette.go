package tui

import (
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Command struct {
	Title       string
	Description string
	Action      func() tea.Msg // Returns a message to dispatch
}

type CommandPalette struct {
	width     int
	height    int
	textInput textinput.Model
	commands  []Command
	filtered  []Command
	selected  int
	active    bool
}

func NewCommandPalette() *CommandPalette {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Focus()
	ti.Prompt = "> "
	ti.CharLimit = 156
	ti.Width = 50

	return &CommandPalette{
		textInput: ti,
		commands:  []Command{}, // To be populated
		filtered:  []Command{},
		selected:  0,
		active:    false,
	}
}

func (m *CommandPalette) SetCommands(cmds []Command) {
	m.commands = cmds
	m.filterCommands()
}

// Dialog Interface Implementation

func (m *CommandPalette) Title() string {
	return "Command Palette"
}

func (m *CommandPalette) Active() bool {
	return m.active
}

func (m *CommandPalette) SetActive(active bool) {
	m.active = active
	if active {
		m.textInput.Focus()
		m.textInput.SetValue("")
		m.filterCommands()
	} else {
		m.textInput.Blur()
	}
}

func (m *CommandPalette) Toggle() {
	m.SetActive(!m.active)
}

func (m *CommandPalette) filterCommands() {
	input := strings.ToLower(m.textInput.Value())
	if input == "" {
		m.filtered = m.commands
	} else {
		m.filtered = []Command{}
		for _, cmd := range m.commands {
			if strings.Contains(strings.ToLower(cmd.Title), input) ||
				strings.Contains(strings.ToLower(cmd.Description), input) {
				m.filtered = append(m.filtered, cmd)
			}
		}
	}
	// Reset selection if out of bounds
	if m.selected >= len(m.filtered) {
		m.selected = 0
	}
}

func (m *CommandPalette) Init() tea.Cmd {
	return textinput.Blink
}

func (m *CommandPalette) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.active = false
			return m, nil
		case "up", "ctrl+k":
			if m.selected > 0 {
				m.selected--
			} else {
				m.selected = len(m.filtered) - 1
			}
		case "down", "ctrl+j":
			if m.selected < len(m.filtered)-1 {
				m.selected++
			} else {
				m.selected = 0
			}
		case "enter":
			if len(m.filtered) > 0 {
				selectedCmd := m.filtered[m.selected]
				m.active = false
				log.Printf("Executed command: %s", selectedCmd.Title)
				if selectedCmd.Action != nil {
					return m, func() tea.Msg { return selectedCmd.Action() }
				}
			}
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	m.filterCommands()

	return m, cmd
}

func (m *CommandPalette) View() string {
	if !m.active {
		return ""
	}

	maxHeight := 8
	listView := ""

	start := m.selected - maxHeight/2
	if start < 0 {
		start = 0
	}
	end := start + maxHeight
	if end > len(m.filtered) {
		end = len(m.filtered)
		// Try to adjust start to show full height if possible
		start = end - maxHeight
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		cmd := m.filtered[i]
		style := StylePaletteItem
		cursor := "  "
		if i == m.selected {
			style = StylePaletteSelected
			cursor = "> "
		}

		item := lipgloss.JoinVertical(lipgloss.Left,
			style.Render(cursor+cmd.Title),
			// Optional: Description could be added here in lighter text
		)
		listView = lipgloss.JoinVertical(lipgloss.Left, listView, item)
	}

	return StylePaletteBox.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.textInput.View(),
			lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(lipgloss.Color(ColorAccent)).Render(""),
			listView,
		),
	)
}
