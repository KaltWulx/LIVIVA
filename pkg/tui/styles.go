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
	ColorBackground = ColorMatrixInk0
	ColorPanel      = ColorMatrixInk1
	ColorBorder     = ColorMatrixInk3

	ColorText      = ColorRainGreenHi
	ColorTextMuted = ColorRainGray

	ColorAccent  = ColorRainGreen // Primary (LIVIVA)
	ColorUser    = ColorRainCyan  // Secondary (User)
	ColorSuccess = ColorRainGreen
	ColorError   = ColorAlertRed
	ColorWarning = ColorRainOrange
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
			Background(lipgloss.Color(ColorPanel)).
			Padding(0, 1).
			Height(1)

	StyleHeaderTitle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorAccent)).
				Bold(true)

	StyleHeaderStatus = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorMatrixInk0)).
				Background(lipgloss.Color(ColorAccent)).
				Padding(0, 1).
				MarginLeft(1).
				Bold(true)

	StyleHeaderMetric = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorTextMuted)).
				Italic(true)

	// Message Blocks

	// User Block: Discrete background + colored left border
	StyleUserBlock = lipgloss.NewStyle().
			Border(lipgloss.Border{
			Left: "┃",
		}, false, false, false, true).
		BorderForeground(lipgloss.Color(ColorUser)).
		Background(lipgloss.Color(ColorPanel)).
		Padding(0, 2).
		MarginTop(1).
		MarginBottom(1)

	// Assistant Block: Clean, Indented, No background
	StyleAssistantBlock = lipgloss.NewStyle().
				Padding(0, 0, 1, 2).
				MarginBottom(1)

	// Assistant Header (LIVIVA label)
	StyleAssistantHeader = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorAccent)).
				Bold(true).
				MarginBottom(0).
				PaddingLeft(1)

	// Input Container
	StyleInputContainer = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true, false, false, false).
				BorderForeground(lipgloss.Color(ColorBorder)).
				PaddingTop(1)

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
)
