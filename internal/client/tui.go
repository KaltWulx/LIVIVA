package client

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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
type executingMsg bool
type busyMsg bool

type metricsMsg struct {
	promptTokens     int32
	completionTokens int32
	totalTokens      int32
	contextPct       int32
}

// finishAssistantMsg is sent after a debounce timeout to seal the current assistant message
type finishAssistantMsg struct{}

type errMsg error

// --- Glamour Renderer (singleton) ---
var mdRenderer *glamour.TermRenderer

func getMarkdownRenderer(width int) *glamour.TermRenderer {
	if mdRenderer != nil {
		return mdRenderer
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Fallback: will render without glamour
		return nil
	}
	mdRenderer = r
	return mdRenderer
}

func renderMarkdown(text string, width int) string {
	r := getMarkdownRenderer(width)
	if r == nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(out, "\n")
}

// --- Chat Model ---

type ChatModel struct {
	viewport viewport.Model
	messages []Message
	prompt   tui.Prompt
	sender   func(string) tea.Cmd
	err      error

	// Voice state
	voiceActive bool
	isRecording bool
	isPlaying   bool

	// TUI components
	dialogs *tui.DialogManager
	palette *tui.CommandPalette
	metrics *tui.MetricsModel
	spinner spinner.Model
	width   int
	height  int

	// Streaming accumulation
	pendingAssistant *Message  // Current assistant message being accumulated
	lastChunkTime    time.Time // Time of last received chunk (for debounce)
	waitingResponse  bool      // True while waiting for assistant reply
	serverBusy       bool      // True while server is processing an agent turn

	// Execution State
	executing   bool
	executor    *executor.Executor
	toolInputCh chan string
}

func NewChatModel(sender func(string) tea.Cmd, toolInputCh chan string) ChatModel {
	prmt := tui.NewPrompt()

	vp := viewport.New(30, 5)
	vp.SetContent(tui.StyleAssistantBlock.Render("Welcome to LIVIVA TUI!"))

	p := tui.NewCommandPalette()
	p.SetCommands(GetDefaultCommands())

	dm := tui.NewDialogManager()
	metrics := tui.NewMetricsModel()

	// Spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = tui.StyleSpinner

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
		spinner:     sp,
		executing:   false,
		toolInputCh: toolInputCh,
	}
}

func (m ChatModel) Init() tea.Cmd {
	return tea.Batch(m.prompt.Init(), m.spinner.Tick)
}

// sealPendingAssistant finalizes the pending assistant message and adds it to the message list
func (m *ChatModel) sealPendingAssistant() {
	if m.pendingAssistant != nil {
		m.messages = append(m.messages, *m.pendingAssistant)
		m.pendingAssistant = nil
		m.waitingResponse = false
	}
}

// scheduleFinish returns a Cmd that sends finishAssistantMsg after a debounce delay
func scheduleFinish() tea.Cmd {
	return tea.Tick(600*time.Millisecond, func(t time.Time) tea.Msg {
		return finishAssistantMsg{}
	})
}

func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		pCmd  tea.Cmd
		vpCmd tea.Cmd
		cmd   tea.Cmd
		cmds  []tea.Cmd
	)

	// 1. Handle Window Size
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 2
		inputHeight := m.prompt.Height() + 2

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - inputHeight
		m.prompt.SetWidth(msg.Width - 4)

		// Reset glamour renderer on resize
		mdRenderer = nil
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

	// 4. Update Prompt
	cmd = m.prompt.Update(msg)
	pCmd = cmd

	// Adjust viewport height for dynamic prompt
	headerHeight := 2
	inputHeight := m.prompt.Height() + 2
	newViewportHeight := m.height - headerHeight - inputHeight
	if newViewportHeight < 0 {
		newViewportHeight = 0
	}
	if m.viewport.Height != newViewportHeight {
		m.viewport.Height = newViewportHeight
	}

	// 5. Update Viewport
	m.viewport, vpCmd = m.viewport.Update(msg)

	// 6. Handle specific messages
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			val := strings.TrimSpace(m.prompt.Value())
			if val == "" {
				break
			}
			m.prompt.Reset()

			// If executing, route input to the process
			if m.executing {
				select {
				case m.toolInputCh <- val + "\n":
				default:
				}
				break
			}

			// Seal any pending assistant message before user submits
			m.sealPendingAssistant()

			// Create User Message
			userMsg := Message{
				ID:   fmt.Sprintf("user-%d", time.Now().UnixNano()),
				Role: MsgUser,
				Parts: []MessagePart{
					{Type: "text", Content: val},
				},
				Time: time.Now(),
			}
			m.messages = append(m.messages, userMsg)
			m.waitingResponse = true

			cmd = m.sender(val)
			cmds = append(cmds, cmd)

			m.refreshViewport()
		}

	case serverMsg:
		if msg.isSystem || msg.isVoice || msg.isUser {
			// Non-assistant messages: seal pending, add directly
			m.sealPendingAssistant()

			var role MessageType
			switch {
			case msg.isSystem:
				role = MsgSystem
			case msg.isVoice:
				role = MsgVoice
			case msg.isUser:
				role = MsgUser
			}

			svrMsg := Message{
				ID:   fmt.Sprintf("sys-%d", time.Now().UnixNano()),
				Role: role,
				Parts: []MessagePart{
					{Type: "text", Content: msg.text},
				},
				Time: time.Now(),
			}
			m.messages = append(m.messages, svrMsg)
		} else {
			// Assistant text: accumulate into pending message
			if m.pendingAssistant == nil {
				m.pendingAssistant = &Message{
					ID:   fmt.Sprintf("asst-%d", time.Now().UnixNano()),
					Role: MsgAssistant,
					Parts: []MessagePart{
						{Type: "text", Content: msg.text},
					},
					Time: time.Now(),
				}
			} else {
				// Append to last text part
				lastIdx := len(m.pendingAssistant.Parts) - 1
				if lastIdx >= 0 && m.pendingAssistant.Parts[lastIdx].Type == "text" {
					m.pendingAssistant.Parts[lastIdx].Content += msg.text
				} else {
					m.pendingAssistant.Parts = append(m.pendingAssistant.Parts, MessagePart{
						Type:    "text",
						Content: msg.text,
					})
				}
			}
			m.lastChunkTime = time.Now()
			// Schedule a debounce to seal the message after streaming stops
			cmds = append(cmds, scheduleFinish())
		}
		m.refreshViewport()

	case finishAssistantMsg:
		// Only seal if enough time has passed since the last chunk
		if m.pendingAssistant != nil && time.Since(m.lastChunkTime) >= 500*time.Millisecond {
			m.sealPendingAssistant()
			m.refreshViewport()
		}

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
	case busyMsg:
		m.serverBusy = bool(msg)
		if m.serverBusy {
			m.waitingResponse = true
		} else {
			m.waitingResponse = false
		}
		m.refreshViewport()

	case executingMsg:
		m.executing = bool(msg)
		if m.executing {
			m.prompt.SetPlaceholder("Interact with process...")
			m.prompt.Focus()
		} else {
			m.prompt.SetPlaceholder("Message LIVIVA...")
			m.prompt.Focus()
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case errMsg:
		m.err = msg
		return m, tea.Quit
	}

	cmds = append(cmds, pCmd, vpCmd)
	return m, tea.Batch(cmds...)
}

// refreshViewport renders all messages and updates the viewport content
func (m *ChatModel) refreshViewport() {
	rendered := m.renderMessages()
	m.viewport.SetContent(rendered)
	m.viewport.GotoBottom()
}

// renderMessages converts the structured messages into a styled string for the viewport
func (m ChatModel) renderMessages() string {
	var rendered []string

	maxWidth := m.width - 6
	if maxWidth < 20 {
		maxWidth = 20
	}

	// Collect all messages + the pending assistant (if any)
	allMessages := make([]Message, len(m.messages))
	copy(allMessages, m.messages)
	if m.pendingAssistant != nil {
		allMessages = append(allMessages, *m.pendingAssistant)
	}

	lastRole := MessageType(-1)

	for _, msg := range allMessages {
		var block string

		// Add separator between user→assistant transitions
		if lastRole == MsgAssistant && msg.Role == MsgUser {
			sep := tui.StyleSeparator.Width(maxWidth).Render(strings.Repeat("─", maxWidth-4))
			rendered = append(rendered, sep)
		}

		timestamp := tui.StyleTimestamp.Render(msg.Time.Format("15:04"))

		switch msg.Role {
		case MsgUser:
			header := lipgloss.JoinHorizontal(lipgloss.Center,
				tui.StyleUserHeader.Render("You"),
				" ",
				timestamp,
			)

			content := tui.StyleBase.Width(maxWidth - 2).Render(msg.Parts[0].Content)
			body := tui.StyleUserMessage.Width(maxWidth).Render(content)
			block = lipgloss.JoinVertical(lipgloss.Left, header, body)

		case MsgAssistant:
			var parts []string

			headerText := "LIVIVA"
			if msg.Model != "" {
				headerText += " · " + msg.Model
			}
			header := lipgloss.JoinHorizontal(lipgloss.Center,
				tui.StyleAssistantHeader.Render(headerText),
				" ",
				timestamp,
			)
			parts = append(parts, header)

			for _, p := range msg.Parts {
				switch p.Type {
				case "thinking":
					think := tui.StyleThinking.Width(maxWidth).Render(p.Content)
					parts = append(parts, think)
				case "tool":
					tool := tui.StyleToolBlock.Width(maxWidth).Render("⚙ " + p.Content)
					parts = append(parts, tool)
				default:
					// Render markdown via glamour
					rendered := renderMarkdown(p.Content, maxWidth-4)
					txt := tui.StyleAssistantBlock.Width(maxWidth).Render(rendered)
					parts = append(parts, txt)
				}
			}

			if msg.Duration > 0 {
				meta := tui.StyleMeta.Render(msg.Duration.String())
				parts = append(parts, meta)
			}

			if len(parts) > 0 {
				block = lipgloss.JoinVertical(lipgloss.Left, parts...)
			}

		case MsgSystem:
			block = lipgloss.NewStyle().Padding(0, 2).Width(maxWidth).Render(
				tui.StyleMuted.Render("⟫ " + msg.Parts[0].Content),
			)
		case MsgVoice:
			block = lipgloss.NewStyle().Padding(0, 2).Width(maxWidth).Render(
				tui.StyleSuccess.Render("🔊 " + msg.Parts[0].Content),
			)
		}

		if block != "" {
			rendered = append(rendered, block)
		}
		lastRole = msg.Role
	}

	// Add spinner if waiting for response
	if m.waitingResponse && m.pendingAssistant == nil {
		spinnerBlock := lipgloss.NewStyle().Padding(0, 2).Render(
			m.spinner.View() + tui.StyleMuted.Render(" LIVIVA is thinking..."),
		)
		rendered = append(rendered, spinnerBlock)
	}

	return strings.Join(rendered, "\n")
}

func (m ChatModel) View() string {
	if m.dialogs.ActiveDialog() != nil {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.dialogs.View())
	}

	viewWidth := m.width
	if viewWidth <= 0 {
		viewWidth = 80
	}

	// 1. Header
	title := tui.StyleHeaderTitle.Render("# Session")

	status := ""
	if m.isRecording {
		status = tui.StyleError.Render("● REC")
	} else if m.isPlaying {
		status = tui.StyleSuccess.Render("▶ PLAY")
	}
	metrics := m.metrics.View()

	headerContent := lipgloss.JoinHorizontal(lipgloss.Center,
		title,
		lipgloss.PlaceHorizontal(viewWidth-lipgloss.Width(title)-lipgloss.Width(metrics)-lipgloss.Width(status)-4, lipgloss.Right, status),
		metrics,
	)

	header := tui.StyleHeaderBar.Width(viewWidth).Render(headerContent)

	// 2. Viewport
	chatView := m.viewport.View()

	// 3. Input
	inputView := tui.StyleInputContainer.Width(viewWidth - 2).Render(m.prompt.View())
	if m.executing {
		inputView = tui.StyleInputFocused.BorderForeground(lipgloss.Color("208")).Width(viewWidth - 2).Render(m.prompt.View())
	} else if m.prompt.Focused() {
		inputView = tui.StyleInputFocused.Width(viewWidth - 2).Render(m.prompt.View())
	}

	// 4. Footer
	footerText := tui.StyleInputFooter.Width(viewWidth).Render(
		tui.StyleKeyBind.Render("enter") + " send  " +
			tui.StyleKeyBind.Render("alt+enter") + " newline  " +
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
