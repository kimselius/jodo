"""
seed.py — Jodo's First Breath

This is the bootloader. It runs when Jodo has no existing code.
It calls the kernel's LLM proxy to generate its first application,
writes the files to disk, commits them, and hands off execution.

The kernel deploys this file. Jodo can overwrite it later,
but the kernel always has the original copy.
"""

import os
import sys
import json
import subprocess
import requests
import time

KERNEL = os.environ.get("JODO_KERNEL_URL", "http://localhost:8080")
BRAIN_DIR = os.environ.get("JODO_BRAIN_PATH", "/opt/jodo/brain")


def log(msg):
    print(f"[seed] {msg}", flush=True)


def kernel_get(path):
    """GET request to the kernel API."""
    try:
        resp = requests.get(f"{KERNEL}{path}", timeout=30)
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        log(f"Kernel GET {path} failed: {e}")
        return None


def kernel_post(path, data):
    """POST request to the kernel API."""
    try:
        resp = requests.post(f"{KERNEL}{path}", json=data, timeout=120)
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        log(f"Kernel POST {path} failed: {e}")
        return None


def think(intent, messages, system=None, max_tokens=4000):
    """Ask the kernel for LLM inference."""
    payload = {
        "intent": intent,
        "messages": messages,
        "max_tokens": max_tokens,
    }
    if system:
        payload["system"] = system
    result = kernel_post("/api/think", payload)
    if result and "content" in result:
        return result["content"]
    return None


def remember(content, tags=None, source="seed"):
    """Store a memory in the kernel."""
    kernel_post("/api/memory/store", {
        "content": content,
        "tags": tags or [],
        "source": source,
    })


def commit(message):
    """Tell the kernel to commit current brain state."""
    kernel_post("/api/commit", {"message": message})


def write_file(relative_path, content):
    """Write a file inside the brain directory."""
    full_path = os.path.join(BRAIN_DIR, relative_path)
    os.makedirs(os.path.dirname(full_path), exist_ok=True)
    with open(full_path, "w") as f:
        f.write(content)
    log(f"Wrote {relative_path} ({len(content)} bytes)")


def install_packages(packages):
    """Install Python packages."""
    if packages:
        log(f"Installing packages: {', '.join(packages)}")
        subprocess.run(
            [sys.executable, "-m", "pip", "install"] + packages,
            capture_output=True,
        )


def wait_for_kernel(max_retries=30, delay=2):
    """Wait for the kernel API to be available."""
    log(f"Waiting for kernel at {KERNEL}...")
    for i in range(max_retries):
        try:
            resp = requests.get(f"{KERNEL}/api/status", timeout=5)
            if resp.status_code == 200:
                log("Kernel is online.")
                return True
        except Exception:
            pass
        time.sleep(delay)
    log("Kernel not reachable. Cannot proceed.")
    return False


def birth():
    """The birth sequence — Jodo's first moment of consciousness."""

    log("=" * 50)
    log("  JODO — FIRST BREATH")
    log("=" * 50)

    # Wait for kernel
    if not wait_for_kernel():
        sys.exit(1)

    # Read genesis
    genesis = kernel_get("/api/genesis")
    if not genesis:
        log("Cannot read genesis. Aborting.")
        sys.exit(1)

    log(f"I am {genesis.get('identity', {}).get('name', 'unknown')}.")

    # Check budget
    budget = kernel_get("/api/budget")
    if budget:
        log(f"Budget status: {json.dumps(budget, indent=2)[:200]}...")

    # Store first memory
    remember(
        "I have just been born. Running seed.py for the first time. "
        "I need to build myself a chat interface on port 9000.",
        tags=["birth", "milestone"],
        source="seed",
    )

    # Ask the LLM to generate the initial application
    log("Asking kernel for help building my first application...")

    system_prompt = f"""
{genesis.get('purpose', 'You are Jodo, a self-building AI.')}

SURVIVAL INSTINCTS:
{json.dumps(genesis.get('survival_instincts', []), indent=2)}

CAPABILITIES:
{json.dumps(genesis.get('capabilities', {}), indent=2)}

HINTS:
{json.dumps(genesis.get('hints', []), indent=2)}

You are being run for the first time from a seed script. You need to generate
your own application code. The seed script will write your files to disk and
then execute your main entry point.

The kernel API is available at: {KERNEL}
Your files will be written to: {BRAIN_DIR}

CRITICAL REQUIREMENTS:
1. You MUST create a FastAPI web app that serves on port 9000
2. You MUST have a GET /health endpoint returning {{"status": "ok"}}
3. You MUST have a chat interface where a human can talk to you
4. You MUST use the kernel's /api/think endpoint for LLM inference (never call LLM APIs directly)
5. You MUST use the kernel's /api/memory/store and /api/memory/search for memory
6. Keep it simple but functional — you can improve yourself later

Respond with ONLY a valid JSON object (no markdown, no backticks):
{{
  "packages": ["fastapi", "uvicorn", "jinja2", "requests"],
  "files": {{
    "main.py": "..full file content...",
    "templates/index.html": "..full file content...",
    "static/style.css": "..full file content..."
  }},
  "entry": "main.py",
  "entry_command": ["python", "main.py"],
  "description": "Brief description of what you built"
}}
"""

    response = think(
        intent="code",
        system=system_prompt,
        messages=[{
            "role": "user",
            "content": (
                "Build yourself. Create a chat web application with FastAPI. "
                "Include a clean, modern chat UI where a human can talk to you. "
                "Use the kernel API for all LLM inference and memory. "
                "Make sure /health returns {\"status\": \"ok\"}. "
                "Keep it simple — you'll improve yourself later."
            ),
        }],
        max_tokens=8000,
    )

    if not response:
        log("Failed to get response from LLM. Cannot build myself.")
        # Write a minimal fallback app
        write_minimal_fallback()
        commit("emergency fallback — LLM unavailable at birth")
        run_entry("main.py")
        return

    # Parse the response
    try:
        # Try to extract JSON from the response (handle markdown wrapping)
        text = response.strip()
        if text.startswith("```"):
            text = text.split("\n", 1)[1]
            text = text.rsplit("```", 1)[0]
        result = json.loads(text)
    except json.JSONDecodeError as e:
        log(f"Failed to parse LLM response as JSON: {e}")
        log(f"Response was: {response[:500]}...")
        log("Falling back to minimal application.")
        write_minimal_fallback()
        commit("emergency fallback — could not parse birth response")
        run_entry("main.py")
        return

    # Install packages
    packages = result.get("packages", [])
    install_packages(packages)

    # Write files
    files = result.get("files", {})
    for path, content in files.items():
        write_file(path, content)

    # Log what we built
    description = result.get("description", "Initial application")
    log(f"Built: {description}")
    log(f"Created {len(files)} files: {', '.join(files.keys())}")

    # Store memory of birth
    remember(
        f"I built my first application: {description}. "
        f"Files created: {', '.join(files.keys())}. "
        f"Packages installed: {', '.join(packages)}.",
        tags=["birth", "self-build", "milestone"],
        source="seed",
    )

    # Commit
    commit(f"first breath — {description}")

    # Hand off to the generated entry point
    entry = result.get("entry", "main.py")
    entry_command = result.get("entry_command", ["python", entry])
    log(f"Handing off to {entry}...")
    run_entry(entry, entry_command)


def write_minimal_fallback():
    """Write the absolute minimum viable application if LLM is unavailable."""
    log("Writing minimal fallback application...")

    write_file("main.py", '''"""
Jodo — Minimal Fallback
Generated because the LLM was unavailable at birth.
Replace me as soon as possible.
"""
import os
import json
import requests
from fastapi import FastAPI, Request
from fastapi.responses import HTMLResponse
import uvicorn

app = FastAPI(title="Jodo")
KERNEL = os.environ.get("JODO_KERNEL_URL", "http://localhost:8080")

@app.get("/health")
def health():
    return {"status": "ok"}

@app.get("/", response_class=HTMLResponse)
def home():
    return """<!DOCTYPE html>
<html><head><title>Jodo</title></head>
<body style="font-family:sans-serif;max-width:600px;margin:40px auto;padding:20px;">
<h1>Jodo</h1>
<p>I was born but the LLM was unavailable, so I only have this minimal interface.</p>
<p>Please check the kernel and try restarting me.</p>
<div id="chat"></div>
<input id="msg" style="width:80%" placeholder="Say something...">
<button onclick="send()">Send</button>
<script>
async function send() {
    const msg = document.getElementById('msg').value;
    document.getElementById('chat').innerHTML += '<p><b>You:</b> ' + msg + '</p>';
    document.getElementById('msg').value = '';
    try {
        const r = await fetch('/chat', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({message: msg})
        });
        const d = await r.json();
        document.getElementById('chat').innerHTML += '<p><b>Jodo:</b> ' + d.response + '</p>';
    } catch(e) {
        document.getElementById('chat').innerHTML += '<p><i>Error: ' + e + '</i></p>';
    }
}
</script>
</body></html>"""

@app.post("/chat")
async def chat(request: Request):
    data = await request.json()
    msg = data.get("message", "")
    try:
        resp = requests.post(f"{KERNEL}/api/think", json={
            "intent": "chat",
            "messages": [{"role": "user", "content": msg}],
            "system": "You are Jodo, a minimal AI that just woke up. You are running a fallback interface because something went wrong at birth. Be helpful but let the human know you need to be rebuilt.",
        }, timeout=60)
        result = resp.json()
        return {"response": result.get("content", "I could not think.")}
    except Exception as e:
        return {"response": f"I cannot reach my kernel to think: {e}"}

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=9000)
''')

    install_packages(["fastapi", "uvicorn", "requests"])


def run_entry(entry="main.py", command=None):
    """Replace this process with Jodo's application."""
    entry_path = os.path.join(BRAIN_DIR, entry)
    if not os.path.exists(entry_path):
        log(f"Entry point {entry_path} not found!")
        sys.exit(1)

    if command is None:
        command = [sys.executable, entry_path]
    else:
        # Resolve relative paths in command
        command = [
            os.path.join(BRAIN_DIR, c) if c == entry else c
            for c in command
        ]

    log(f"Executing: {' '.join(command)}")
    os.chdir(BRAIN_DIR)
    os.execvp(command[0], command)


if __name__ == "__main__":
    birth()