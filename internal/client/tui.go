package client

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kalt/liviva/pkg/tui"
)

// Internal message types for bubbletea
type serverMsg struct {
	text     string
	isSystem bool
	isVoice  bool
}

type recordingMsg bool
type playingMsg bool

type metricsMsg struct {
	promptTokens     int32
	completionTokens int32
	totalTokens      int32
	contextPct       int32
}

type errMsg error

type model struct {
	viewport    viewport.Model
	messages    []string
	textarea    textarea.Model
	sender      func(string) tea.Cmd // Function to send to gRPC stream asynchronously
	err         error
	voiceActive bool
	isRecording bool
	isPlaying   bool

	// New TUI components
	palette tui.CommandPalette
	width   int
	height  int

	// Model metrics
	promptTokens     int32
	completionTokens int32
	totalTokens      int32
	contextPct       int32
}

func initialModel(sender func(string) tea.Cmd) model {
	ta := textarea.New()
	ta.Placeholder = "Send a message... (Ctrl+P for commands)"
	ta.Focus()
	ta.Prompt = "│ "
	ta.CharLimit = 2000
	ta.SetWidth(30)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	vp := viewport.New(30, 5)
	vp.SetContent(tui.StyleAssistantBlock.Render("Welcome to LIVIVA TUI!"))

	p := tui.NewCommandPalette()
	p.SetCommands(GetDefaultCommands())

	return model{
		textarea:    ta,
		viewport:    vp,
		messages:    []string{},
		sender:      sender,
		voiceActive: false,
		isRecording: false,
		isPlaying:   false,
		palette:     p,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		cmd   tea.Cmd
	)

	// 1. Handle Window Size first to ensure child models have correct dimensions
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 1
		// Calculate input height based on lines in textarea + borders
		inputLines := m.textarea.Height() // current height of textarea
		inputHeight := inputLines + 3     // textarea + top border + help text line

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - inputHeight
		m.textarea.SetWidth(msg.Width - 4)
	}

	// 2. Handle Palette Interaction
	if m.palette.IsActive() {
		var pCmd tea.Cmd
		m.palette, pCmd = m.palette.Update(msg)
		return m, pCmd
	}

	// 3. Global Key Handling for Palette Toggle
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+p" {
			m.palette.Toggle()
			return m, nil
		}
	}

	// 4. Normal Interaction
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			val := strings.TrimSpace(m.textarea.Value())
			if val == "" {
				break
			}

			// Render User Message Block
			contentWidth := m.width - 10
			if contentWidth < 20 {
				contentWidth = 20
			}
			wrappedContent := lipgloss.NewStyle().Width(contentWidth).Render(val)
			userBlock := tui.StyleUserBlock.Render(wrappedContent)
			m.messages = append(m.messages, userBlock)

			cmd = m.sender(val)
			m.textarea.Reset()
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
		}

	case serverMsg:
		var content string
		contentWidth := m.width - 10
		if contentWidth < 20 {
			contentWidth = 20
		}

		if msg.isSystem {
			wrapped := lipgloss.NewStyle().Width(contentWidth).Render("[System] " + msg.text)
			content = tui.StyleAssistantBlock.Foreground(lipgloss.Color(tui.ColorTextMuted)).Render(wrapped)
		} else if msg.isVoice {
			wrapped := lipgloss.NewStyle().Width(contentWidth).Render("[Voice] " + msg.text)
			content = tui.StyleAssistantBlock.Foreground(lipgloss.Color(tui.ColorSuccess)).Render(wrapped)
		} else {
			header := tui.StyleAssistantHeader.Render("▣ LIVIVA")
			wrappedBody := lipgloss.NewStyle().Width(contentWidth).Render(msg.text)
			body := tui.StyleAssistantBlock.Render(wrappedBody)
			content = lipgloss.JoinVertical(lipgloss.Left, header, body)
		}

		m.messages = append(m.messages, content)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case metricsMsg:
		m.promptTokens = msg.promptTokens
		m.completionTokens = msg.completionTokens
		m.totalTokens = msg.totalTokens
		m.contextPct = msg.contextPct

	case recordingMsg:
		m.isRecording = bool(msg)
	case playingMsg:
		m.isPlaying = bool(msg)
	case errMsg:
		m.err = msg
		return m, tea.Quit
	}

	return m, tea.Batch(tiCmd, vpCmd, cmd)
}

func (m model) View() string {
	if m.palette.IsActive() {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.palette.View())
	}

	// Safe Width fallback
	viewWidth := m.width
	if viewWidth <= 0 {
		viewWidth = 80
	}

	// 1. Header Construction (Polished Matrix Style)
	title := tui.StyleHeaderTitle.Render("LIVIVA")

	status := ""
	if m.isRecording {
		status = tui.StyleHeaderStatus.Render("REC")
	} else if m.isPlaying {
		status = tui.StyleHeaderStatus.Render("PLAY")
	}

	metrics := tui.StyleHeaderMetric.Render(fmt.Sprintf("%dT | %d%%ctx", m.totalTokens, m.contextPct))

	// Create the horizontal bar
	leftSide := lipgloss.JoinHorizontal(lipgloss.Center, title, status)

	// Join left and right side with flexible spacing
	headerContent := lipgloss.PlaceHorizontal(viewWidth, lipgloss.Left,
		leftSide,
		lipgloss.WithWhitespaceForeground(lipgloss.Color(tui.ColorPanel)),
		lipgloss.WithWhitespaceBackground(lipgloss.Color(tui.ColorPanel)),
	)

	// Manually place metrics on the right if space allows
	if viewWidth > lipgloss.Width(leftSide)+lipgloss.Width(metrics)+2 {
		mWidth := lipgloss.Width(metrics)
		headerContent = headerContent[:viewWidth-mWidth] + metrics
	}

	header := tui.StyleHeaderBar.Width(viewWidth - 2).Render(headerContent)

	// 2. Viewport
	chatView := m.viewport.View()

	// 3. Input Container
	inputParams := tui.StyleMuted.Render(" (Ctrl+P for commands, Ctrl+C to quit)")
	inputBlock := tui.StyleInputContainer.Width(viewWidth - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			m.textarea.View(),
			inputParams,
		),
	)

	// Combine components
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		chatView,
		inputBlock,
	)
}
