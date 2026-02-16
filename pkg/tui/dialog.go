package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Dialog is the interface for all modal components
type Dialog interface {
	tea.Model
	Title() string
	Active() bool
	SetActive(bool)
}

// DialogManager handles the stack of active dialogs
type DialogManager struct {
	activeDialog Dialog
}

func NewDialogManager() *DialogManager {
	return &DialogManager{}
}

func (dm *DialogManager) ActiveDialog() Dialog {
	return dm.activeDialog
}

func (dm *DialogManager) Open(d Dialog) {
	if dm.activeDialog != nil {
		dm.activeDialog.SetActive(false)
	}
	dm.activeDialog = d
	d.SetActive(true)
}

func (dm *DialogManager) Close() {
	if dm.activeDialog != nil {
		dm.activeDialog.SetActive(false)
		dm.activeDialog = nil
	}
}

func (dm *DialogManager) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if dm.activeDialog != nil && dm.activeDialog.Active() {
		newModel, cmd := dm.activeDialog.Update(msg)
		if newDialog, ok := newModel.(Dialog); ok {
			dm.activeDialog = newDialog
			// If the dialog deactivated itself during Update, close it
			if !dm.activeDialog.Active() {
				dm.activeDialog = nil
			}
		}
		return nil, cmd
	}
	return nil, nil
}

func (dm *DialogManager) View() string {
	if dm.activeDialog != nil && dm.activeDialog.Active() {
		return dm.activeDialog.View()
	}
	return ""
}
