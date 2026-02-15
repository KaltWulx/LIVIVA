package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// StdioTransport implements mcp.Transport for local subprocesses.
type StdioTransport struct {
	command string
	args    []string
}

// NewStdioTransport creates a transport factory.
func NewStdioTransport(command string, args []string) *StdioTransport {
	return &StdioTransport{
		command: command,
		args:    args,
	}
}

// Connect starts the subprocess and returns a connection.
func (t *StdioTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	cmd := exec.CommandContext(ctx, t.command, t.args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	conn := &StdioConnection{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		readCh: make(chan jsonrpc.Message, 10), // Buffer slightly
		errCh:  make(chan error, 1),
	}

	// Start background readers
	go conn.readLoop()
	go conn.readStderr()

	return conn, nil
}

// StdioConnection implements mcp.Connection.
type StdioConnection struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	readCh chan jsonrpc.Message
	errCh  chan error

	mu     sync.Mutex
	closed bool
}

// Read implements mcp.Connection.Read
func (c *StdioConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	select {
	case msg := <-c.readCh:
		return msg, nil
	case err := <-c.errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Write implements mcp.Connection.Write
func (c *StdioConnection) Write(ctx context.Context, message jsonrpc.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("connection closed")
	}

	// Helper to marshal JSON-RPC message
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Append newline for JSON-RPC over stdio
	// This assumes the receiver reads lines.
	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

// Close implements mcp.Connection.Close
func (c *StdioConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Close pipes
	_ = c.stdin.Close()
	// Stdout/Stderr are closed by read loops or process exit usually,
	// but strictly we can't close ReadEnd of pipe safely if Read is blocked,
	// however here Read is in a goroutine that will return EOF when process dies.

	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return nil
}

// SessionID implements mcp.Connection.SessionID
// For stdio, we don't have a specific session ID, usually returning empty or random is fine.
func (c *StdioConnection) SessionID() string {
	return ""
}

func (c *StdioConnection) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	// Increase buffer size
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		msgBytes := scanner.Bytes()

		// Decode using SDK helper
		msg, err := jsonrpc.DecodeMessage(msgBytes)
		if err != nil {
			// Log decode error but don't kill connection necessarily?
			// Actually MCP is strict.
			// For now, let's try to continue or send error?
			// Sending error to errCh might kill the consumer loop.
			// Let's print to stderr and continue if possible, or fail.
			fmt.Printf("[MCP-STDIO] Decode error: %v\n", err)
			continue
		}

		select {
		case c.readCh <- msg:
		case <-c.errCh: // If error already happened
			return
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case c.errCh <- err:
		default:
		}
	} else {
		// EOF
		select {
		case c.errCh <- io.EOF:
		default:
		}
	}

	// Close implicitly
	c.Close()
}

func (c *StdioConnection) readStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		fmt.Printf("[MCP-STDERR] %s\n", scanner.Text())
	}
}
