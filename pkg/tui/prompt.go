package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Prompt is a wrapper around bubbles/textarea with enhanced styling and functionality
type Prompt struct {
	textarea textarea.Model
	focused  bool
	// Future: History, Autocomplete state
}

func NewPrompt() Prompt {
	ta := textarea.New()
	ta.Placeholder = "Ask anything... (Enter to send, Alt+Enter for new line)"
	ta.Focus()
	ta.Prompt = " "  // We'll handle the prompt rendering via styles
	ta.CharLimit = 0 // Unlimited
	ta.SetWidth(50)  // Will be resized
	ta.SetHeight(1)  // Auto-expanding
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false) // We handle this manually if needed

	// Styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = StyleInputArea
	ta.BlurredStyle.Base = StyleInputArea

	return Prompt{
		textarea: ta,
		focused:  true,
	}
}

func (p Prompt) Init() tea.Cmd {
	return textarea.Blink
}

func (p *Prompt) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.textarea, cmd = p.textarea.Update(msg)

	// Auto-resize
	lineCount := p.textarea.LineCount()
	if lineCount < 1 {
		lineCount = 1
	}
	// Cap max height to avoid eating too much screen
	const MaxHeight = 5
	if lineCount > MaxHeight {
		lineCount = MaxHeight
	}

	if p.textarea.Height() != lineCount {
		p.textarea.SetHeight(lineCount)
	}

	return cmd
}

func (p Prompt) View() string {
	return p.textarea.View()
}

func (p *Prompt) Value() string {
	return p.textarea.Value()
}

func (p *Prompt) SetValue(v string) {
	p.textarea.SetValue(v)
}

func (p *Prompt) Reset() {
	p.textarea.Reset()
}

func (p *Prompt) Focus() tea.Cmd {
	p.focused = true
	return p.textarea.Focus()
}

func (p *Prompt) Blur() {
	p.focused = false
	p.textarea.Blur()
}

func (p *Prompt) Focused() bool {
	return p.focused
}

func (p *Prompt) SetWidth(w int) {
	p.textarea.SetWidth(w)
}

func (p *Prompt) Height() int {
	return p.textarea.Height()
}
