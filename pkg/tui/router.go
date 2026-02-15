package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ViewState defines the current active screen
type ViewState int

const (
	ViewHome ViewState = iota
	ViewSession
	ViewHelp
)

// Router manages the navigation between different TUI views
type Router struct {
	CurrentView ViewState
	Width       int
	Height      int
}

func NewRouter() *Router {
	return &Router{
		CurrentView: ViewHome,
	}
}

func (r *Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Router logic mainly handles global navigation commands
	// But individual views will likely handle their own updates
	return nil, nil // Placeholder
}

func (r *Router) SetView(view ViewState) {
	r.CurrentView = view
}

func (r *Router) Resize(width, height int) {
	r.Width = width
	r.Height = height
}
