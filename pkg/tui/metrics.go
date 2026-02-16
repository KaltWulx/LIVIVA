package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type MetricsMsg struct {
	PromptTokens     int32
	CompletionTokens int32
	TotalTokens      int32
	ContextPct       int32
}

type MetricsModel struct {
	PromptTokens     int32
	CompletionTokens int32
	TotalTokens      int32
	ContextPct       int32
}

func NewMetricsModel() *MetricsModel {
	return &MetricsModel{}
}

func (m *MetricsModel) Init() tea.Cmd {
	return nil
}

func (m *MetricsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MetricsMsg:
		m.PromptTokens = msg.PromptTokens
		m.CompletionTokens = msg.CompletionTokens
		m.TotalTokens = msg.TotalTokens
		m.ContextPct = msg.ContextPct
	}
	return m, nil
}

func (m *MetricsModel) View() string {
	return StyleHeaderMetric.Render(fmt.Sprintf("%dT | %d%%ctx", m.TotalTokens, m.ContextPct))
}
