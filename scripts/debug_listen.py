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
edge_tts_path = shutil.which("edge-tts")
if not edge_tts_path:
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
                os.killpg(os.getpgid(current_speech_process.pid), signal.SIGTERM)
            except ProcessLookupError:
                pass 
            except Exception as e:
                sys.stderr.write(f"Error stopping speech: {e}\n")
            current_speech_process = None
    is_agent_speaking.clear()

def speak(text):
    """Synthesize speech from text using edge-tts piped to mpv."""
    global current_speech_process
    
    if not text.strip():
        return
    
    clean_text = re.sub(r'\[([^\]]+)\]\([^)]+\)', r'\1', text) 
    clean_text = re.sub(r'[*_`#]', '', clean_text)            
    clean_text = clean_text.strip()
    
    if not clean_text:
        return

    sys.stderr.write(f"Speaking: {clean_text}\n")
    
    quoted_text = shlex.quote(clean_text)

    # SPLIT: Generate to file, then play
    audio_file = "/tmp/debug_audio.mp3"
    gen_cmd = f"{shlex.quote(edge_tts_path)} --text {quoted_text} --voice es-ES-AlvaroNeural --write-media {audio_file}"
    
    stop_speaking() 
    
    is_agent_speaking.set()
    with speech_lock:
        try:
            sys.stderr.write(f"Generating audio to {audio_file}...\n")
            # Step 1: Generate
            gen_proc = subprocess.run(gen_cmd, shell=True, stderr=None)
            if gen_proc.returncode != 0:
                 sys.stderr.write(f"Generation failed with code {gen_proc.returncode}\n")
                 is_agent_speaking.clear()
                 return

            sys.stderr.write(f"Playing audio with mpv...\n")
            # Step 2: Play
            # Use --msg-level=all=warn to see errors
            play_cmd = f"mpv --no-terminal --msg-level=all=warn {audio_file}"
            current_speech_process = subprocess.Popen(
                play_cmd, 
                shell=True, 
                preexec_fn=os.setsid,
                stderr=None 
            )
        except Exception as e:
            sys.stderr.write(f"Failed to start TTS: {e}\n")
            is_agent_speaking.clear()
            return

    if current_speech_process:
        current_speech_process.wait()
    is_agent_speaking.clear()

def listen_loop():
    """Continuously listen for speech and print to stdout."""
    # CHANGED: Reduced adjust duration for faster startup
    with sr.Microphone() as source:
        sys.stderr.write("Adjusting for ambient noise... (Please wait)\n")
        recognizer.adjust_for_ambient_noise(source, duration=0.5)
        
        while not shutdown_flag.is_set():
            try:
                sys.stderr.write("Listening...\n")
                audio = recognizer.listen(source, timeout=None, phrase_time_limit=5)
                
                if is_agent_speaking.is_set():
                    sys.stderr.write("Ignoring input (Agent is speaking)...\n")
                    continue

                sys.stderr.write("Processing audio...\n")
                try:
                    text = recognizer.recognize_google(audio, language="es-ES")
                    
                    if is_agent_speaking.is_set():
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
            speak(clean_text)
        except ValueError:
            break
        except Exception as e:
            if not shutdown_flag.is_set():
                sys.stderr.write(f"Error in output loop: {e}\n")

if __name__ == "__main__":
    t_out = threading.Thread(target=output_loop, daemon=True)
    t_out.start()
    listen_loop()
