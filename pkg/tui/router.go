package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ViewState defines the current active screen
type ViewState int

const (
	ViewHome ViewState = iota
	ViewSession
	ViewSettings
)

type NavigateMsg ViewState

// Router manages the navigation between different TUI views
type Router struct {
	CurrentView ViewState
	ctx         *Context
}

func NewRouter(ctx *Context) *Router {
	return &Router{
		CurrentView: ViewHome,
		ctx:         ctx,
	}
}

func (r *Router) Init() tea.Cmd {
	return nil
}

func (r *Router) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case NavigateMsg:
		r.CurrentView = ViewState(msg)
	}
	return r, nil
}

func (r *Router) View() string {
	return "" // This will typically be called by the main model's View to decide what to render
}
