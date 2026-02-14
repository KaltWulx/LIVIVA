import sys
import threading
import time
import re
import signal
import subprocess
import os
import shutil
import shlex

try:
    import speech_recognition as sr
except ImportError:
    sys.stderr.write("Error: speech_recognition not installed. Run: pip install SpeechRecognition pyaudio\n")
    sys.exit(1)

# Check for edge-tts
# Since we might be running from a venv without activation, look near the python executable first
edge_tts_path = shutil.which("edge-tts")
if not edge_tts_path:
    # Try finding it in the same directory as the python executable
    possible_path = os.path.join(os.path.dirname(sys.executable), "edge-tts")
    if os.path.isfile(possible_path) and os.access(possible_path, os.X_OK):
        edge_tts_path = possible_path

if not edge_tts_path:
    sys.stderr.write("Error: edge-tts not found. Run: pip install edge-tts\n")
    sys.exit(1)

# Check for mpv
if not shutil.which("mpv"):
    sys.stderr.write("Error: mpv not found. Please install it (e.g., sudo pacman -S mpv)\n")
    sys.exit(1)

# Initialize recognizer
recognizer = sr.Recognizer()

# Global state
shutdown_flag = threading.Event()
current_speech_process = None
speech_lock = threading.Lock()
is_agent_speaking = threading.Event()

def signal_handler(sig, frame):
    sys.stderr.write("\nExiting...\n")
    stop_speaking()
    shutdown_flag.set()
    sys.exit(0)

signal.signal(signal.SIGINT, signal_handler)

def stop_speaking():
    """Stop the current TTS process immediately."""
    global current_speech_process
    with speech_lock:
        if current_speech_process:
            try:
                # Kill the process group to ensure mpv AND edge-tts die
                os.killpg(os.getpgid(current_speech_process.pid), signal.SIGTERM)
            except ProcessLookupError:
                pass # Already dead
            except Exception as e:
                sys.stderr.write(f"Error stopping speech: {e}\n")
            current_speech_process = None
    is_agent_speaking.clear()

def speak(text):
    """Synthesize speech from text using edge-tts piped to mpv."""
    global current_speech_process
    
    if not text.strip():
        return
    
    # Strip markdown
    clean_text = re.sub(r'\[([^\]]+)\]\([^)]+\)', r'\1', text) # Links
    clean_text = re.sub(r'[*_`#]', '', clean_text)            # Basic markdown chars
    clean_text = clean_text.strip()
    
    if not clean_text:
        return

    sys.stderr.write(f"Speaking: {clean_text}\n")
    
    # Securely quote the text for shell execution
    quoted_text = shlex.quote(clean_text)
    
    # SPLIT: Generate to file, then play
    # This avoids pipe issues and allows better error handling/logging
    audio_file = "/tmp/liviva_speech.mp3"
    
    # Step 1: Generate audio
    gen_cmd = f"{shlex.quote(edge_tts_path)} --text {quoted_text} --voice es-MX-DaliaNeural --write-media {audio_file}"
    
    stop_speaking() # Ensure previous is stopped
    
    is_agent_speaking.set()
    with speech_lock:
        try:
            sys.stderr.write(f"[TTS] Generating audio...\n")
            # Run generation synchronously
            gen_proc = subprocess.run(gen_cmd, shell=True, stderr=None) # stderr=None lets errors show
            
            if gen_proc.returncode != 0:
                 sys.stderr.write(f"[TTS] Generation failed with code {gen_proc.returncode}\n")
                 is_agent_speaking.clear()
                 return

            sys.stderr.write(f"[TTS] Playing audio...\n")
            
            # Step 2: Play audio
            # --no-terminal: don't clutter stdout
            # --msg-level=all=warn: show warnings/errors
            play_cmd = f"mpv --no-terminal --msg-level=all=warn {audio_file}"
            
            current_speech_process = subprocess.Popen(
                play_cmd, 
                shell=True, 
                preexec_fn=os.setsid,
                stderr=None # Show mpv errors
            )
        except Exception as e:
            sys.stderr.write(f"[TTS] Failed to start: {e}\n")
            is_agent_speaking.clear()
            return

    # Wait for completion, but allow barge-in to kill it
    if current_speech_process:
        current_speech_process.wait()
    is_agent_speaking.clear()

def listen_loop():
    """Continuously listen for speech and print to stdout."""
    with sr.Microphone() as source:
        sys.stderr.write("Adjusting for ambient noise... (Please wait)\n")
        recognizer.adjust_for_ambient_noise(source, duration=1)
        # Increase pause threshold to be less sensitive to natural pauses in speech
        recognizer.pause_threshold = 1.5 
        
        while not shutdown_flag.is_set():
            try:
                sys.stderr.write("Listening...\n")
                # Wait for speech
                audio = recognizer.listen(source, timeout=None, phrase_time_limit=15)
                
                # HALF-DUPLEX CHECK:
                # If agent was speaking during capture, discard it (high probability of self-feedback)
                if is_agent_speaking.is_set():
                    sys.stderr.write("Ignoring input (Agent is speaking)...\n")
                    continue

                sys.stderr.write("Processing audio...\n")
                try:
                    # Recognize speech using Google Speech Recognition
                    text = recognizer.recognize_google(audio, language="es-ES")
                    
                    # Double check just in case race condition
                    if is_agent_speaking.is_set():
                        sys.stderr.write("Ignoring verified text (Agent started speaking)...\n")
                        continue

                    if text:
                        print(text)
                        sys.stdout.flush()
                        sys.stderr.write(f"You said: {text}\n")
                except sr.UnknownValueError:
                    pass
                except sr.RequestError as e:
                    sys.stderr.write(f"Could not request results; {e}\n")
            
            except Exception as e:
                # If we catch an error related to interrupting wait, just continue
                if not shutdown_flag.is_set():
                    sys.stderr.write(f"Error in listen loop: {e}\n")
                time.sleep(1)

def output_loop():
    """Read from stdin (Go app output) and speak it."""
    while not shutdown_flag.is_set():
        try:
            line = sys.stdin.readline()
            if not line:
                break
            
            clean_text = line.strip()
            # Remove "Agent: " prefix if present
            clean_text = re.sub(r'^(Agent|LIVIVA):\s*', '', clean_text, flags=re.IGNORECASE)
            
            # Skip empty lines or known log patterns
            if not clean_text or clean_text.startswith("202") or "Using GitHub Copilot" in clean_text:
                continue
                
            speak(clean_text)
        except ValueError:
            break
        except Exception as e:
            if not shutdown_flag.is_set():
                sys.stderr.write(f"Error in output loop: {e}\n")

if __name__ == "__main__":
    # Start output thread first
    t_out = threading.Thread(target=output_loop, daemon=True)
    t_out.start()

    # Start listening in main thread
    listen_loop()
