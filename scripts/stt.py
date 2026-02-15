import sys
import threading
import speech_recognition as sr
import signal
import time

# Initialize recognizer
recognizer = sr.Recognizer()
shutdown_flag = threading.Event()

def signal_handler(sig, frame):
    shutdown_flag.set()
    sys.exit(0)

signal.signal(signal.SIGINT, signal_handler)

def listen_loop():
    """Continuously listen for speech and print to stdout."""
    try:
        with sr.Microphone() as source:
            sys.stderr.write("Adjusting for ambient noise...\n")
            recognizer.adjust_for_ambient_noise(source, duration=0.5)
            recognizer.pause_threshold = 1.2
            
            while not shutdown_flag.is_set():
                try:
                    sys.stderr.write("Listening...\n")
                    audio = recognizer.listen(source, timeout=None, phrase_time_limit=10)
                    
                    sys.stderr.write("Processing...\n")
                    try:
                        text = recognizer.recognize_google(audio, language="es-ES")
                        if text:
                            print(text)
                            sys.stdout.flush()
                    except sr.UnknownValueError:
                        pass
                    except sr.RequestError as e:
                        sys.stderr.write(f"STT Error: {e}\n")
                except Exception as e:
                    if not shutdown_flag.is_set():
                        sys.stderr.write(f"Loop Error: {e}\n")
                    time.sleep(0.5)
    except Exception as e:
        sys.stderr.write(f"Fatal Error: {e}\n")
        sys.exit(1)

if __name__ == "__main__":
    listen_loop()
