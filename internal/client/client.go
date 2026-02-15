package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kalt/liviva/pkg/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	sttCmd     *exec.Cmd
	sttMu      sync.Mutex
	sttRunning bool
)

// Run starts the client CLI
func Run(addr string) {
	log.Printf("Connecting to LIVIVA Server at %s...", addr)

	// Dial Server
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := api.NewLivivaServiceClient(conn)

	// Start Chat Session
	ctx := context.Background()
	stream, err := c.ChatSession(ctx)
	if err != nil {
		log.Fatalf("error creating stream: %v", err)
	}

	waitc := make(chan struct{})

	// Receiver Goroutine (Server -> Client)
	go func() {
		defer close(waitc)
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Fatalf("Failed to receive: %v", err)
			}

			switch p := in.Payload.(type) {
			case *api.ServerMessage_Text:
				// Print clean response
				fmt.Printf("\r\033[K%s\nUser -> ", p.Text)
			case *api.ServerMessage_SystemLog:
				// Print system log safely
				fmt.Printf("\r\033[K[System] %s\nUser -> ", p.SystemLog)
			case *api.ServerMessage_SpeakText:
				// Trigger local TTS
				fmt.Printf("\r\033[K[Voice] %s\nUser -> ", p.SpeakText)
				go speak(p.SpeakText)
			}
		}
	}()

	// Input Loop (Client -> Server)
	fmt.Print("User -> ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()

		// Local Command Detection
		if strings.HasPrefix(text, "/voice on") {
			startSTT(stream)
		} else if strings.HasPrefix(text, "/voice off") {
			stopSTT()
		}

		// Send Text
		if err := stream.Send(&api.ClientMessage{
			Payload: &api.ClientMessage_Text{
				Text: text,
			},
		}); err != nil {
			log.Fatalf("Failed to send: %v", err)
		}
		fmt.Print("User -> ")
	}
	stream.CloseSend()
	stopSTT() // Ensure STT is off
	<-waitc
}

// startSTT launches the microphone listener script
func startSTT(stream api.LivivaService_ChatSessionClient) {
	sttMu.Lock()
	defer sttMu.Unlock()

	if sttRunning {
		return
	}

	// Resolution of bins: Check .venv first, then PATH
	cwd, _ := os.Getwd()
	venvPython := filepath.Join(cwd, ".venv", "bin", "python3")
	pythonPath := "python3"
	if _, err := os.Stat(venvPython); err == nil {
		pythonPath = venvPython
	}

	scriptPath := "./scripts/stt.py"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptPath = filepath.Join(cwd, "scripts", "stt.py")
	}

	cmd := exec.Command(pythonPath, scriptPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("STT Error: failed to get stdout pipe: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("STT Error: failed to start stt.py: %v", err)
		return
	}

	sttCmd = cmd
	sttRunning = true
	log.Println("\r\033[K[Client] Microphone Service Started (Local)")

	// Goroutine to read transcribed text
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			transcribed := scanner.Text()
			if transcribed != "" {
				fmt.Printf("\r\033[K(Mic) %s\nUser -> ", transcribed)
				stream.Send(&api.ClientMessage{
					Payload: &api.ClientMessage_Text{
						Text: transcribed,
					},
				})
			}
		}
		sttMu.Lock()
		sttRunning = false
		sttMu.Unlock()
	}()
}

// stopSTT kills the microphone listener script
func stopSTT() {
	sttMu.Lock()
	defer sttMu.Unlock()

	if !sttRunning || sttCmd == nil {
		return
	}

	if err := sttCmd.Process.Signal(os.Interrupt); err != nil {
		sttCmd.Process.Kill()
	}
	sttCmd.Wait()
	sttRunning = false
	log.Println("\r\033[K[Client] Microphone Service Stopped")
}

// speak executes local TTS and plays audio
func speak(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	// Resolution of bins: Check .venv first, then PATH
	cwd, _ := os.Getwd()
	venvEdge := filepath.Join(cwd, ".venv", "bin", "edge-tts")
	edgeTTsPath, err := exec.LookPath("edge-tts")
	if err != nil {
		// Fallback to .venv
		if _, statErr := os.Stat(venvEdge); statErr == nil {
			edgeTTsPath = venvEdge
		} else {
			log.Printf("Voice Error: edge-tts not found in PATH or .venv/bin")
			return
		}
	}

	// We use a pipe to send edge-tts output to mpv
	// edge-tts --text "..." --voice es-MX-DaliaNeural --write-media - | mpv --no-terminal -

	ttsCmd := exec.Command(edgeTTsPath, "--text", text, "--voice", "es-MX-DaliaNeural", "--write-media", "-")
	playCmd := exec.Command("mpv", "--no-terminal", "-")

	reader, writer := io.Pipe()
	ttsCmd.Stdout = writer
	playCmd.Stdin = reader

	if err := ttsCmd.Start(); err != nil {
		log.Printf("TTS Start Error: %v", err)
		return
	}
	if err := playCmd.Start(); err != nil {
		log.Printf("Playback Start Error: %v", err)
		return
	}

	go func() {
		ttsCmd.Wait()
		writer.Close()
	}()

	playCmd.Wait()
	reader.Close()
}
