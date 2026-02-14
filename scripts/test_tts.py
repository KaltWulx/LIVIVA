import subprocess
import shutil
import shlex
import sys
import os

def test_speak(text):
    edge_tts_path = shutil.which("edge-tts")
    if not edge_tts_path:
        possible_path = os.path.join(os.path.dirname(sys.executable), "edge-tts")
        if os.path.isfile(possible_path) and os.access(possible_path, os.X_OK):
            edge_tts_path = possible_path
    
    if not edge_tts_path:
        print("Error: edge-tts not found")
        return

    print(f"Using edge-tts at: {edge_tts_path}")
    
    if not shutil.which("mpv"):
        print("Error: mpv not found")
        return

    clean_text = text.strip()
    quoted_text = shlex.quote(clean_text)
    
    cmd = f"{shlex.quote(edge_tts_path)} --text {quoted_text} --voice es-ES-AlvaroNeural --write-media - | mpv --no-terminal -"
    print(f"Executing: {cmd}")
    
    try:
        process = subprocess.Popen(
            cmd, 
            shell=True, 
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )
        stdout, stderr = process.communicate()
        print("STDOUT:", stdout.decode())
        print("STDERR:", stderr.decode())
    except Exception as e:
        print(f"Failed to run TTS: {e}")

if __name__ == "__main__":
    test_speak("Hola, esta es una prueba de audio.")
