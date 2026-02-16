package client

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kalt/liviva/pkg/api"
)

type VoiceService struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	running bool
	mu      sync.Mutex
	program *tea.Program
}

func NewVoiceService() *VoiceService {
	return &VoiceService{}
}

func (s *VoiceService) SetProgram(p *tea.Program) {
	s.program = p
}

func (s *VoiceService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *VoiceService) Start(stream api.LivivaService_ChatSessionClient, sendMu *sync.Mutex) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	// Resolution of bins: Check .venv first, then PATH
	cwd, _ := os.Getwd()
	venvPython := filepath.Join(cwd, ".venv", "bin", "python3")
	pythonPath := "python3"
	if _, err := os.Stat(venvPython); err == nil {
		pythonPath = venvPython
	}

	scriptPath := "./scripts/listen.py"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptPath = filepath.Join(cwd, "scripts", "listen.py")
	}

	cmd := exec.Command(pythonPath, scriptPath)

	// IMPORTANT: Pipes must be handled carefully to avoid TUI corruption
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.logError(fmt.Sprintf("failed to get stdout pipe: %v", err))
		return
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.logError(fmt.Sprintf("failed to get stdin pipe: %v", err))
		return
	}
	s.stdin = stdin

	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.logError(fmt.Sprintf("failed to get stderr pipe: %v", err))
		return
	}

	if err := cmd.Start(); err != nil {
		s.logError(fmt.Sprintf("failed to start stt.py: %v", err))
		return
	}

	s.cmd = cmd
	s.running = true
	if s.program != nil {
		s.program.Send(recordingMsg(true))
	}

	// Goroutine to read stderr (Logging)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// Write to log file, NOT stdout
			log.Printf("STT Debug: %s", scanner.Text())
		}
	}()

	// Goroutine to read transcribed text
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			transcribed := scanner.Text()
			if transcribed != "" {
				if transcribed == "[SPEAKING] START" {
					if s.program != nil {
						s.program.Send(playingMsg(true))
					}
					continue
				}
				if transcribed == "[SPEAKING] END" {
					if s.program != nil {
						s.program.Send(playingMsg(false))
					}
					continue
				}

				if s.program != nil {
					s.program.Send(serverMsg{text: transcribed, isUser: true})
				}

				sendMu.Lock()
				stream.Send(&api.ClientMessage{
					Payload: &api.ClientMessage_Text{
						Text: transcribed,
					},
				})
				sendMu.Unlock()
			}
		}
		s.Stop()
	}()
}

func (s *VoiceService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.cmd == nil {
		return
	}

	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		s.cmd.Process.Kill()
	}
	s.cmd.Wait()
	s.running = false
	s.stdin = nil

	if s.program != nil {
		s.program.Send(recordingMsg(false))
	}
}

func (s *VoiceService) Speak(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.stdin == nil {
		log.Printf("Voice Warning: Skip speaking, voice mode is off")
		return
	}

	fmt.Fprintln(s.stdin, text)
}

func (s *VoiceService) logError(msg string) {
	log.Printf("STT Error: %s", msg)
	if s.program != nil {
		s.program.Send(serverMsg{text: "STT Error: " + msg, isSystem: true})
	}
}
