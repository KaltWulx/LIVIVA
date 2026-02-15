package client

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kalt/liviva/pkg/tui"
)

func GetDefaultCommands() []tui.Command {
	return []tui.Command{
		{
			Title:       "Toggle Voice Mode",
			Description: "Enable or disable voice input and output",
			Action: func() tea.Msg {
				// return generic msg to be handled by update loop
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}, Alt: true} // Simulate Alt+V or custom msg
			},
		},
		{
			Title:       "Connect Provider",
			Description: "Select an LLM provider (OpenAI, Anthropic, etc.)",
			Action: func() tea.Msg {
				return nil // TODO: Implement provider switching
			},
		},
		{
			Title:       "Switch Model",
			Description: "Change the active language model",
			Action: func() tea.Msg {
				return nil // TODO: Implement model switching
			},
		},
		{
			Title:       "Upload File",
			Description: "Send a local file to the agent",
			Action: func() tea.Msg {
				// This would ideally open a file picker, but for now we just
				// prompt the user to type /upload in the chat
				return serverMsg{text: "Type /upload <path> to send a file.", isSystem: true}
			},
		},
		{
			Title:       "Quit",
			Description: "Exit LIVIVA",
			Action: func() tea.Msg {
				return tea.Quit()
			},
		},
	}
}
