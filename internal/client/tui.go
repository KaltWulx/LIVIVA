package client

import (
	"strings"

	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	executor "github.com/kalt/liviva/internal/client/exec"
	"github.com/kalt/liviva/pkg/tui"
)

// Message Types for Structured Rendering
type MessageType int

const (
	MsgUser MessageType = iota
	MsgAssistant
	MsgSystem
	MsgVoice
)

type MessagePart struct {
	Type     string // "text", "thinking", "tool"
	Content  string
	Metadata map[string]string
}

type Message struct {
	ID       string
	Role     MessageType
	Parts    []MessagePart
	Model    string
	Duration time.Duration
	Time     time.Time
}

// Internal message types for bubbletea
type serverMsg struct {
	text     string
	isSystem bool
	isVoice  bool
	isUser   bool
}

type recordingMsg bool
type playingMsg bool
type executingMsg bool // New message type

type metricsMsg struct {
	promptTokens     int32
	completionTokens int32
	totalTokens      int32
	contextPct       int32
}

type errMsg error

type ChatModel struct {
	viewport    viewport.Model
	messages    []Message // Changed from []string
	prompt      tui.Prompt
	sender      func(string) tea.Cmd
	err         error
	voiceActive bool
	isRecording bool
	isPlaying   bool

	// New TUI components
	dialogs *tui.DialogManager
	palette *tui.CommandPalette
	metrics *tui.MetricsModel
	width   int
	height  int

	// Execution State
	executing   bool
	executor    *executor.Executor
	toolInputCh chan string
}

func NewChatModel(sender func(string) tea.Cmd, toolInputCh chan string) ChatModel {
	// Use new Prompt
	prmt := tui.NewPrompt()

	vp := viewport.New(30, 5)
	vp.SetContent(tui.StyleAssistantBlock.Render("Welcome to LIVIVA TUI!"))

	p := tui.NewCommandPalette()
	p.SetCommands(GetDefaultCommands())

	dm := tui.NewDialogManager()
	metrics := tui.NewMetricsModel()

	return ChatModel{
		prompt:      prmt,
		viewport:    vp,
		messages:    []Message{},
		sender:      sender,
		voiceActive: false,
		isRecording: false,
		isPlaying:   false,
		palette:     p,
		dialogs:     dm,
		metrics:     metrics,
		executing:   false,
		toolInputCh: toolInputCh,
	}
}

func (m ChatModel) Init() tea.Cmd {
	return m.prompt.Init()
}

func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		pCmd  tea.Cmd
		vpCmd tea.Cmd
		cmd   tea.Cmd
	)

	// 1. Handle Window Size
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 2                    // Minimal header
		inputHeight := m.prompt.Height() + 2 // + borders/padding

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - inputHeight
		m.prompt.SetWidth(msg.Width - 4)
	}

	// 2. Handle Dialogs
	if m.dialogs.ActiveDialog() != nil {
		_, cmd := m.dialogs.Update(msg)
		return m, cmd
	}

	// 3. Global Key Handling
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+p" {
			if m.palette.Active() {
				m.dialogs.Close()
			} else {
				m.dialogs.Open(m.palette)
			}
			return m, nil
		}
	}

	// 4. Normal Interaction
	// Update Prompt
	cmd = m.prompt.Update(msg)
	pCmd = cmd

	// Check if Prompt height changed and adjust Viewport
	headerHeight := 2
	inputHeight := m.prompt.Height() + 2
	newViewportHeight := m.height - headerHeight - inputHeight
	if newViewportHeight < 0 {
		newViewportHeight = 0
	}
	if m.viewport.Height != newViewportHeight {
		m.viewport.Height = newViewportHeight
	}

	// Update Viewport (with new height)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			// Check for Alt+Enter (if supported by terminal) or just check for escaping?
			// Bubbles textarea handles Ctrl+J as newline if configured, but we disabled valid newlines in Enter for now?
			// Actually, we want Enter to submit, and maybe Alt+Enter to insert newline.
			// Standard bubbles behavior: Shift+Enter or Ctrl+J often triggers newline if InsertNewline is set.
			// But we disabled InsertNewline in NewPrompt keymap.

			val := strings.TrimSpace(m.prompt.Value())
			if val == "" {
				break
			}
			m.prompt.Reset()

			// If executing, route input to the process
			if m.executing {
				// Don't show in chat (PTY will echo if needed, or hide if password)
				// Send to executor channel
				select {
				case m.toolInputCh <- val + "\n": // Add newline as standard input usually expects it
				default:
				}
				break
			}

			// Create User Message
			userMsg := Message{
				ID:   time.Now().String(), // Simple ID
				Role: MsgUser,
				Parts: []MessagePart{
					{Type: "text", Content: val},
				},
				Time: time.Now(),
			}
			m.messages = append(m.messages, userMsg)

			cmd = m.sender(val)

			// Render all messages
			rendered := m.renderMessages()
			m.viewport.SetContent(rendered)
			m.viewport.GotoBottom()
		}

	case serverMsg:
		// Create Server Message
		var role MessageType
		if msg.isSystem {
			role = MsgSystem
		} else if msg.isVoice {
			role = MsgVoice
		} else if msg.isUser {
			role = MsgUser
		} else {
			role = MsgAssistant
		}

		svrMsg := Message{
			ID:   time.Now().String(),
			Role: role,
			Parts: []MessagePart{
				{Type: "text", Content: msg.text},
			},
			Time: time.Now(),
		}

		m.messages = append(m.messages, svrMsg)

		rendered := m.renderMessages()
		m.viewport.SetContent(rendered)
		m.viewport.GotoBottom()

	case metricsMsg:
		m.metrics.Update(tui.MetricsMsg{
			PromptTokens:     max(0, msg.promptTokens),
			CompletionTokens: max(0, msg.completionTokens),
			TotalTokens:      max(0, msg.totalTokens),
			ContextPct:       max(0, msg.contextPct),
		})

	case recordingMsg:
		m.isRecording = bool(msg)
	case playingMsg:
		m.isPlaying = bool(msg)
	case executingMsg:
		m.executing = bool(msg)
		if m.executing {
			m.prompt.SetPlaceholder("Interact with process...")
			m.prompt.Focus()
		} else {
			m.prompt.SetPlaceholder("Message LIVIVA...")
			m.prompt.Focus()
		}
	case errMsg:
		m.err = msg
		return m, tea.Quit
	}

	return m, tea.Batch(pCmd, vpCmd, cmd)
}

// renderMessages converts the structured messages into a string for the viewport
func (m ChatModel) renderMessages() string {
	var rendered []string

	// Ensure we have enough width
	maxWidth := m.width - 6
	if maxWidth < 20 {
		maxWidth = 20
	}

	for _, msg := range m.messages {
		var block string
		switch msg.Role {
		case MsgUser:
			// OpenCode User Style:
			// Header (You) + Content Block with Left Border
			header := tui.StyleUserHeader.Render("You")

			// Content is rendered inside the block style
			content := tui.StyleBase.Width(maxWidth - 2).Render(msg.Parts[0].Content)
			body := tui.StyleUserMessage.Width(maxWidth).Render(content)

			// Join vertical
			block = lipgloss.JoinVertical(lipgloss.Left, header, body)

		case MsgAssistant:
			// OpenCode Assistant Style:
			// Header (LIVIVA) + Clean blocks
			var parts []string

			// Header with identity and metadata (e.g. model)
			headerText := "LIVIVA"
			if msg.Model != "" {
				headerText += " · " + msg.Model
			}
			header := tui.StyleAssistantHeader.Render(headerText)
			parts = append(parts, header)

			for _, p := range msg.Parts {
				if p.Type == "thinking" {
					// Thinking block
					think := tui.StyleThinking.Width(maxWidth).Render(p.Content)
					parts = append(parts, think)
				} else if p.Type == "tool" {
					// Tool block
					tool := tui.StyleToolBlock.Width(maxWidth).Render("⚙ " + p.Content)
					parts = append(parts, tool)
				} else {
					// Standard Text
					// Render text cleanly with correct width constraints
					txt := tui.StyleAssistantBlock.Width(maxWidth).Render(p.Content)
					parts = append(parts, txt)
				}
			}

			// Footer handling if needed (e.g. Duration)
			if msg.Duration > 0 {
				meta := tui.StyleMeta.Render(msg.Duration.String())
				parts = append(parts, meta)
			}

			if len(parts) > 0 {
				block = lipgloss.JoinVertical(lipgloss.Left, parts...)
			}

		case MsgSystem:
			block = lipgloss.NewStyle().Padding(0, 2).Width(maxWidth).Render(
				tui.StyleMuted.Render("System: " + msg.Parts[0].Content),
			)
		case MsgVoice:
			block = lipgloss.NewStyle().Padding(0, 2).Width(maxWidth).Render(
				tui.StyleSuccess.Render("Voice: " + msg.Parts[0].Content),
			)
		}
		if block != "" {
			rendered = append(rendered, block)
		}
	}

	return strings.Join(rendered, "\n")
}

func (m ChatModel) View() string {
	if m.dialogs.ActiveDialog() != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.dialogs.View())
	}

	// Safe Width fallback
	viewWidth := m.width
	if viewWidth <= 0 {
		viewWidth = 80
	}

	// 1. Header (OpenCode Minimalist)
	// Left: Title
	title := tui.StyleHeaderTitle.Render("# Session")

	// Right: Metrics + Status
	status := ""
	if m.isRecording {
		status = tui.StyleError.Render("REC")
	} else if m.isPlaying {
		status = tui.StyleSuccess.Render("PLAY")
	}
	metrics := m.metrics.View() // Use the metrics component view (make sure existing view style matches)

	// Join
	headerContent := lipgloss.JoinHorizontal(lipgloss.Center,
		title,
		lipgloss.PlaceHorizontal(viewWidth-lipgloss.Width(title)-lipgloss.Width(metrics)-lipgloss.Width(status)-4, lipgloss.Right, status),
		metrics,
	)

	header := tui.StyleHeaderBar.Width(viewWidth).Render(headerContent)

	// 2. Viewport
	chatView := m.viewport.View()

	// 3. Input
	// We use the prompt's View which includes the textarea
	// Wrap it in container style
	inputView := tui.StyleInputContainer.Width(viewWidth - 2).Render(m.prompt.View())
	if m.executing {
		inputView = tui.StyleInputFocused.BorderForeground(lipgloss.Color("208")).Width(viewWidth - 2).Render(m.prompt.View()) // Orange border for Exec
		m.prompt.SetPlaceholder("Interact with process...")
	} else if m.prompt.Focused() {
		inputView = tui.StyleInputFocused.Width(viewWidth - 2).Render(m.prompt.View())
		m.prompt.SetPlaceholder("Message LIVIVA...")
	}

	// Combine components
	footerText := tui.StyleInputFooter.Width(viewWidth).Render(
		tui.StyleKeyBind.Render("ctrl+t") + " variants  " +
			tui.StyleKeyBind.Render("tab") + " agents  " +
			tui.StyleKeyBind.Render("ctrl+p") + " commands",
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		chatView,
		inputView,
		footerText,
	)
}

// AppModel is the top-level model that manages routing
type AppModel struct {
	router *tui.Router
	chat   ChatModel
	ctx    *tui.Context
}

func NewAppModel(sender func(string) tea.Cmd, toolInputCh chan string) AppModel {
	ctx := &tui.Context{}
	router := tui.NewRouter(ctx)
	chat := NewChatModel(sender, toolInputCh)

	return AppModel{
		router: router,
		chat:   chat,
		ctx:    ctx,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.chat.Init(),
		m.router.Init(),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Update Context on WindowSize
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.ctx.Width = msg.Width
		m.ctx.Height = msg.Height
	}

	// Handle Navigation
	if _, ok := msg.(tui.NavigateMsg); ok {
		var cmd tea.Cmd
		newUserModel, cmd := m.router.Update(msg)
		if r, ok := newUserModel.(*tui.Router); ok {
			m.router = r
		}
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	// Route based on current view
	switch m.router.CurrentView {
	case tui.ViewHome:
		// For now, Home is Chat
		newChat, cmd := m.chat.Update(msg)
		if c, ok := newChat.(ChatModel); ok {
			m.chat = c
		}
		cmds = append(cmds, cmd)
	case tui.ViewSession:
		newChat, cmd := m.chat.Update(msg)
		if c, ok := newChat.(ChatModel); ok {
			m.chat = c
		}
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	switch m.router.CurrentView {
	case tui.ViewHome:
		return m.chat.View()
	case tui.ViewSession:
		return m.chat.View()
	default:
		return "Unknown View"
	}
}
