package executor

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// Executor handles command execution with PTY support
type Executor struct {
	cmd  *exec.Cmd
	ptmx *os.File
}

// NewExecutor creates a new executor
func NewExecutor() *Executor {
	return &Executor{}
}

// Start executes the command in a pseudo-terminal.
// It returns the ptmx (master) file which can be used to read/write to the process.
func (e *Executor) Start(command []string) (*os.File, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("no command provided")
	}

	e.cmd = exec.Command(command[0], command[1:]...)

	// Start the command with a PTY
	ptmx, err := pty.Start(e.cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}
	e.ptmx = ptmx

	// Handle window size resizing if needed (optional for now)
	return ptmx, nil
}

// Wait waits for the command to finish
func (e *Executor) Wait() error {
	if e.cmd == nil {
		return fmt.Errorf("command not started")
	}
	return e.cmd.Wait()
}

// Close closes the ptmx file
func (e *Executor) Close() {
	if e.ptmx != nil {
		_ = e.ptmx.Close()
	}
}

// WriteInput writes input to the process stdin via PTY
func (e *Executor) WriteInput(data []byte) error {
	if e.ptmx == nil {
		return fmt.Errorf("process not running")
	}
	_, err := e.ptmx.Write(data)
	return err
}

// Resize resizes the PTY (useful if TUI resizes)
func (e *Executor) Resize(rows, cols int) error {
	if e.ptmx == nil {
		return nil
	}
	return pty.Setsize(e.ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}
