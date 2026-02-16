package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// OpenCode "Matrix" Theme Palette
const (
	// Defs
	ColorMatrixInk0 = "#0a0e0a" // background
	ColorMatrixInk1 = "#0e130d" // panel
	ColorMatrixInk2 = "#141c12" // element/subtle
	ColorMatrixInk3 = "#1e2a1b" // border

	ColorRainGreen    = "#2eff6a" // primary / borderActive
	ColorRainGreenHi  = "#62ff94" // text
	ColorRainGreenDim = "#1cc24b" // success dim
	ColorRainCyan     = "#00efff" // secondary
	ColorRainPurple   = "#c770ff" // accent
	ColorRainGray     = "#8ca391" // muted / context
	ColorRainOrange   = "#ffa83d" // warning/emph
	ColorAlertRed     = "#ff4b4b" // error

	// Semantic Mapping
	ColorBackground   = ColorMatrixInk0
	ColorPanel        = ColorMatrixInk1
	ColorBorder       = ColorMatrixInk3
	ColorBorderActive = ColorRainGreen

	ColorText      = ColorRainGreenHi
	ColorTextMuted = ColorRainGray

	ColorAccent   = ColorRainGreen // Primary (LIVIVA)
	ColorUser     = ColorRainCyan  // Secondary (User)
	ColorSuccess  = ColorRainGreen
	ColorError    = ColorAlertRed
	ColorWarning  = ColorRainOrange
	ColorThinking = ColorRainGray
	ColorMeta     = ColorMatrixInk2
)

var (
	// Base Styles
	StyleBase  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))
	StyleMuted = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextMuted))
	StyleBold  = lipgloss.NewStyle().Bold(true)

	// Status Styles
	StyleError   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	StyleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	StyleWarning = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning))

	// Header Styles
	StyleHeaderBar = lipgloss.NewStyle().
			Padding(0, 1). // Minimal padding, no background by default
			Height(1)

	StyleHeaderTitle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorText)).
				Bold(true).
				MarginRight(1)

	StyleHeaderMetric = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorTextMuted))

	// Message Blocks

	// User Block: Discrete
	StyleUserBlock = lipgloss.NewStyle().
			Padding(0, 1).
			MarginTop(0)

	StyleUserHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorUser)).
			Bold(true).
			MarginLeft(2). // Align with padding
			MarginBottom(0)

	// Assistant Block: Clean
	StyleAssistantBlock = lipgloss.NewStyle().
				Padding(0, 1).
				MarginBottom(1)

	// Assistant Header (LIVIVA label)
	StyleAssistantHeader = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorAccent)).
				Bold(true).
				MarginLeft(1).
				MarginBottom(0)

	// Input Area
	StyleInputContainer = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorBorder)).
				Padding(0, 1)

	StyleInputFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorBorderActive)).
				Padding(0, 1)

	StyleInputArea = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorText))

	// Command Palette Styles (Adapting to new colors)
	StylePaletteBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorAccent)).
			Padding(0, 1).
			Width(60)

	StylePaletteItem = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color(ColorText))

	StylePaletteSelected = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color(ColorAccent)).
				Background(lipgloss.Color(ColorPanel)).
				Bold(true)

	// New Message Styles (OpenCode Authentic)
	StyleUserMessage = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true). // Left border only
				BorderForeground(lipgloss.Color(ColorUser)).
				Padding(0, 1).
				MarginTop(0).
				Background(lipgloss.Color(ColorPanel))

	StyleThinking = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Foreground(lipgloss.Color(ColorTextMuted)).
			Padding(0, 1).
			MarginLeft(1).
			MarginTop(0).
			Italic(true)

	StyleMeta = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorTextMuted)).
			MarginTop(0).
			PaddingLeft(1)

	StyleToolBlock = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Padding(0, 1).
			MarginLeft(1).
			MarginTop(0)

	StyleInputFooter = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorTextMuted)).
				MarginTop(0).
				Align(lipgloss.Right)

	StyleKeyBind = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAccent)).
			Bold(true)

	// Separator between conversation turns
	StyleSeparator = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBorder)).
			MarginTop(1).
			MarginBottom(1)

	// Timestamp for messages
	StyleTimestamp = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorTextMuted)).
			Italic(true)

	// Spinner / Thinking indicator
	StyleSpinner = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorRainGreen)).
			Bold(true)
)
