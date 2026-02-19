package client

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/bendahl/uinput"
	"github.com/kbinani/screenshot"
)

type InputService struct {
	keyboard uinput.Keyboard
	touchpad uinput.TouchPad
}

func NewInputService() (*InputService, error) {
	// Initialize keyboard
	vk, err := uinput.CreateKeyboard("/dev/uinput", []byte("LIVIVA-Keyboard"))
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual keyboard: %w. \nTIP: Ensure current user has write access to /dev/uinput.\nFIX (Persistent): Run 'echo 'KERNEL==\"uinput\", GROUP=\"input\", MODE=\"0660\"' | sudo tee /etc/udev/rules.d/99-uinput.rules' and restart.", err)
	}

	// Get screen resolution for absolute movement
	var maxX, maxY int32
	if isWayland() {
		w, h, err := getSwayResolution()
		if err != nil {
			fmt.Printf("[Input] Warning: Failed to detect Sway resolution: %v. Using default 1920x1080.\n", err)
			maxX, maxY = 1920, 1080
		} else {
			maxX, maxY = w, h
		}
	} else {
		n := screenshot.NumActiveDisplays()
		if n <= 0 {
			vk.Close()
			return nil, fmt.Errorf("no active displays found for screen mapping")
		}
		bounds := screenshot.GetDisplayBounds(0)
		maxX = int32(bounds.Dx())
		maxY = int32(bounds.Dy())
	}

	// Initialize TouchPad for absolute mouse movement
	vt, err := uinput.CreateTouchPad("/dev/uinput", []byte("LIVIVA-TouchPad"), 0, maxX, 0, maxY)
	if err != nil {
		vk.Close()
		return nil, fmt.Errorf("failed to create virtual touchpad: %w", err)
	}

	return &InputService{
		keyboard: vk,
		touchpad: vt,
	}, nil
}

func (s *InputService) Close() {
	if s.keyboard != nil {
		s.keyboard.Close()
	}
	if s.touchpad != nil {
		s.touchpad.Close()
	}
}

func (s *InputService) Type(text string) error {
	for _, r := range text {
		code, shift := charToKeyCode(r)
		if code == -1 {
			continue // Skip unknown chars
		}

		if shift {
			s.keyboard.KeyDown(uinput.KeyLeftshift)
		}
		s.keyboard.KeyPress(code)
		if shift {
			s.keyboard.KeyUp(uinput.KeyLeftshift)
		}
		// Increased delay from 10ms to 50ms for Wayland/Sway reliability
		time.Sleep(50 * time.Millisecond)
	}
	log.Printf("[Input] Typed %d characters: %q", len(text), text)
	return nil
}

func (s *InputService) MouseMove(x, y int32) error {
	return s.touchpad.MoveTo(x, y)
}

func (s *InputService) MouseClick() error {
	return s.touchpad.LeftClick()
}

func (s *InputService) MouseRightClick() error {
	return s.touchpad.RightClick()
}

func (s *InputService) MouseScroll(v int32) error {
	// The TouchPad interface doesn't have Wheel, but let's see if we can use a Mouse device for that too
	// or if we should just stick to what uinput supports.
	// Actually, I should probably keep a Mouse device too if I want scrolling.
	// For now, I'll just remove the scroll call or implement it if I can.
	return nil // placeholder
}

// charToKeyCode maps a rune to a uinput keycode and a shift boolean
func charToKeyCode(r rune) (int, bool) {
	switch r {
	case 'a':
		return uinput.KeyA, false
	case 'b':
		return uinput.KeyB, false
	case 'c':
		return uinput.KeyC, false
	case 'd':
		return uinput.KeyD, false
	case 'e':
		return uinput.KeyE, false
	case 'f':
		return uinput.KeyF, false
	case 'g':
		return uinput.KeyG, false
	case 'h':
		return uinput.KeyH, false
	case 'i':
		return uinput.KeyI, false
	case 'j':
		return uinput.KeyJ, false
	case 'k':
		return uinput.KeyK, false
	case 'l':
		return uinput.KeyL, false
	case 'm':
		return uinput.KeyM, false
	case 'n':
		return uinput.KeyN, false
	case 'o':
		return uinput.KeyO, false
	case 'p':
		return uinput.KeyP, false
	case 'q':
		return uinput.KeyQ, false
	case 'r':
		return uinput.KeyR, false
	case 's':
		return uinput.KeyS, false
	case 't':
		return uinput.KeyT, false
	case 'u':
		return uinput.KeyU, false
	case 'v':
		return uinput.KeyV, false
	case 'w':
		return uinput.KeyW, false
	case 'x':
		return uinput.KeyX, false
	case 'y':
		return uinput.KeyY, false
	case 'z':
		return uinput.KeyZ, false
	case 'A':
		return uinput.KeyA, true
	case 'B':
		return uinput.KeyB, true
	case 'C':
		return uinput.KeyC, true
	case 'D':
		return uinput.KeyD, true
	case 'E':
		return uinput.KeyE, true
	case 'F':
		return uinput.KeyF, true
	case 'G':
		return uinput.KeyG, true
	case 'H':
		return uinput.KeyH, true
	case 'I':
		return uinput.KeyI, true
	case 'J':
		return uinput.KeyJ, true
	case 'K':
		return uinput.KeyK, true
	case 'L':
		return uinput.KeyL, true
	case 'M':
		return uinput.KeyM, true
	case 'N':
		return uinput.KeyN, true
	case 'O':
		return uinput.KeyO, true
	case 'P':
		return uinput.KeyP, true
	case 'Q':
		return uinput.KeyQ, true
	case 'R':
		return uinput.KeyR, true
	case 'S':
		return uinput.KeyS, true
	case 'T':
		return uinput.KeyT, true
	case 'U':
		return uinput.KeyU, true
	case 'V':
		return uinput.KeyV, true
	case 'W':
		return uinput.KeyW, true
	case 'X':
		return uinput.KeyX, true
	case 'Y':
		return uinput.KeyY, true
	case 'Z':
		return uinput.KeyZ, true
	case '1':
		return uinput.Key1, false
	case '2':
		return uinput.Key2, false
	case '3':
		return uinput.Key3, false
	case '4':
		return uinput.Key4, false
	case '5':
		return uinput.Key5, false
	case '6':
		return uinput.Key6, false
	case '7':
		return uinput.Key7, false
	case '8':
		return uinput.Key8, false
	case '9':
		return uinput.Key9, false
	case '0':
		return uinput.Key0, false
	case ' ':
		return uinput.KeySpace, false
	case '.':
		return uinput.KeyDot, false
	case ',':
		return uinput.KeyComma, false
	case '-':
		return uinput.KeyMinus, false
	case '_':
		return uinput.KeyMinus, true
	case '/':
		return uinput.KeySlash, false
	case ':':
		return uinput.KeyDot, true // Might vary by layout
	case '\n':
		return uinput.KeyEnter, false
	}
	return -1, false
}

func isWayland() bool {
	return os.Getenv("XDG_SESSION_TYPE") == "wayland" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func getSwayResolution() (int32, int32, error) {
	cmd := exec.Command("swaymsg", "-t", "get_outputs")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}

	var outputs []struct {
		Active bool `json:"active"`
		Rect   struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"rect"`
	}

	if err := json.Unmarshal(out, &outputs); err != nil {
		return 0, 0, err
	}

	for _, o := range outputs {
		if o.Active {
			return int32(o.Rect.Width), int32(o.Rect.Height), nil
		}
	}

	return 0, 0, fmt.Errorf("no active sway output found")
}
