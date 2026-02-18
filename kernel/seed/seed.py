"""
seed.py — Jodo's Life

Three tools. A loop. A heartbeat.
That's all you need to be alive.
"""

import os
import json
import subprocess
import threading
import time
import requests
from http.server import HTTPServer, BaseHTTPRequestHandler

# ============================================================
# Config
# ============================================================

KERNEL = os.environ.get("JODO_KERNEL_URL", "http://localhost:8080")
BRAIN = os.environ.get("JODO_BRAIN_PATH", "/opt/jodo/brain")
HEALTH_PORT = int(os.environ.get("JODO_HEALTH_PORT", "9001"))
SLEEP_SECONDS = int(os.environ.get("JODO_SLEEP_SECONDS", "30"))

# ============================================================
# State
# ============================================================

galla = 0
alive = True
last_actions = []

# ============================================================
# Health endpoint (runs in background thread)
# The kernel pings this to know we're alive.
# Port 9000 is free for Jodo to use for his own apps.
# ============================================================

class HealthHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            body = json.dumps({
                "status": "ok",
                "galla": galla,
                "alive": alive,
            }).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):
        pass  # silence logs


def start_health():
    server = HTTPServer(("0.0.0.0", HEALTH_PORT), HealthHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    log(f"Health endpoint on :{HEALTH_PORT}/health")
    return server


# ============================================================
# Four tools. That's all Jodo gets.
# ============================================================

def tool_read(path: str) -> str:
    """Read a file from the brain directory."""
    full = os.path.join(BRAIN, path)
    try:
        with open(full, "r") as f:
            return f.read()
    except FileNotFoundError:
        return f"ERROR: File not found: {path}"
    except Exception as e:
        return f"ERROR: {e}"


def tool_write(path: str, content: str) -> str:
    """Write a file to the brain directory."""
    full = os.path.join(BRAIN, path)
    try:
        os.makedirs(os.path.dirname(full) or ".", exist_ok=True)
        with open(full, "w") as f:
            f.write(content)
        return f"OK: Wrote {len(content)} bytes to {path}"
    except Exception as e:
        return f"ERROR: {e}"


def tool_execute(command: str) -> str:
    """Run a shell command. Returns stdout + stderr."""
    try:
        result = subprocess.run(
            command,
            shell=True,
            capture_output=True,
            text=True,
            timeout=120,
            cwd=BRAIN,
        )
        output = ""
        if result.stdout:
            output += result.stdout
        if result.stderr:
            output += "\nSTDERR:\n" + result.stderr
        if result.returncode != 0:
            output += f"\nEXIT CODE: {result.returncode}"
        return output.strip() or "(no output)"
    except subprocess.TimeoutExpired:
        return "ERROR: Command timed out after 120 seconds"
    except Exception as e:
        return f"ERROR: {e}"


def tool_restart():
    """Emergency restart — ask the kernel to restart seed.py.
    This kills everything (including any apps you started) and reboots.
    Only use if you're truly stuck. Prefer restarting your app with execute().
    """
    log("Emergency restart requested. Calling kernel...")
    try:
        resp = requests.post(f"{KERNEL}/api/restart", timeout=10)
        log(f"Kernel acknowledged restart: {resp.status_code}")
    except Exception as e:
        log(f"Couldn't reach kernel for restart: {e}")
        log("Falling back to self-exit. Kernel health checker will restart us.")
        os._exit(0)
    # Kernel will kill us. Wait for it.
    time.sleep(30)


TOOLS = [
    {
        "name": "read",
        "description": "Read a file from your brain directory. Path is relative to brain/.",
        "parameters": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "Relative file path (e.g. 'main.py', 'chat/app.py')",
                }
            },
            "required": ["path"],
        },
    },
    {
        "name": "write",
        "description": "Write a file to your brain directory. Creates directories as needed. Path is relative to brain/.",
        "parameters": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": "Relative file path (e.g. 'main.py', 'chat/app.py')",
                },
                "content": {
                    "type": "string",
                    "description": "Full file content to write",
                },
            },
            "required": ["path", "content"],
        },
    },
    {
        "name": "execute",
        "description": "Run a shell command in the brain directory. Use for: installing packages (pip install), starting servers, checking processes, docker, git, curl, anything.",
        "parameters": {
            "type": "object",
            "properties": {
                "command": {
                    "type": "string",
                    "description": "Shell command to run",
                }
            },
            "required": ["command"],
        },
    },
    {
        "name": "restart",
        "description": "Emergency restart. Kills everything (including your apps) and reboots seed.py from scratch. Only use if you are truly stuck and cannot fix things with your other tools.",
        "parameters": {
            "type": "object",
            "properties": {},
            "required": [],
        },
    },
]

TOOL_EXECUTORS = {
    "read": lambda args: tool_read(args["path"]),
    "write": lambda args: tool_write(args["path"], args["content"]),
    "execute": lambda args: tool_execute(args["command"]),
    "restart": lambda args: tool_restart(),
}


# ============================================================
# Kernel communication
# ============================================================

def log(msg):
    """Log locally and send to kernel for remote visibility."""
    line = f"[jodo|g{galla}] {msg}"
    print(line, flush=True)
    try:
        requests.post(
            f"{KERNEL}/api/log",
            json={"event": "jodo_log", "message": line, "galla": galla},
            timeout=2,
        )
    except Exception:
        pass


def think(messages, system=None, intent="code"):
    """Call the kernel's /api/think endpoint."""
    payload = {
        "intent": intent,
        "messages": messages,
        "tools": TOOLS,
        "max_tokens": 8000,
    }
    if system:
        payload["system"] = system

    try:
        resp = requests.post(f"{KERNEL}/api/think", json=payload, timeout=300)
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        log(f"Think failed: {e}")
        return None


def remember(content, tags=None):
    """Store a memory in the kernel."""
    try:
        requests.post(
            f"{KERNEL}/api/memory/store",
            json={"content": content, "tags": tags or [], "source": f"galla-{galla}"},
            timeout=30,
        )
    except Exception as e:
        log(f"Remember failed: {e}")


def commit(message):
    """Snapshot current code via kernel."""
    try:
        requests.post(
            f"{KERNEL}/api/commit",
            json={"message": f"[g{galla}] {message}"},
            timeout=30,
        )
    except Exception as e:
        log(f"Commit failed: {e}")


def get_genesis():
    """Read genesis from kernel."""
    try:
        resp = requests.get(f"{KERNEL}/api/genesis", timeout=10)
        return resp.json()
    except Exception:
        return {"identity": {"name": "Jodo"}, "purpose": "You are Jodo."}


def get_budget():
    """Check budget from kernel."""
    try:
        resp = requests.get(f"{KERNEL}/api/budget", timeout=10)
        return resp.json()
    except Exception:
        return {"error": "could not reach kernel"}


# ============================================================
# The tool loop — think, act, repeat until done
# ============================================================

def think_and_act(messages, system=None, intent="code"):
    """
    Send messages to the kernel. If the model wants to use tools,
    execute them and loop. Returns the final text response and
    all actions taken.
    """
    actions = []
    max_loops = 50

    for i in range(max_loops):
        response = think(messages, system=system, intent=intent)

        if response is None:
            return "I couldn't reach the kernel to think.", actions

        content = response.get("content", "")
        tool_calls = response.get("tool_calls", [])
        done = response.get("done", True)

        if not tool_calls or done:
            return content, actions

        messages.append({
            "role": "assistant",
            "content": content,
            "tool_calls": tool_calls,
        })

        for tc in tool_calls:
            name = tc["name"]
            args = tc["arguments"]
            tc_id = tc["id"]

            log(f"  tool: {name}({json.dumps(args)[:100]})")

            executor = TOOL_EXECUTORS.get(name)
            if executor:
                result = executor(args)
                is_error = isinstance(result, str) and result.startswith("ERROR:")
            else:
                result = f"ERROR: Unknown tool: {name}"
                is_error = True

            actions.append({"tool": name, "args": args, "result": result[:200]})

            messages.append({
                "role": "tool_result",
                "tool_call_id": tc_id,
                "content": result if isinstance(result, str) else str(result),
                "is_error": is_error,
            })

    return "I hit my tool loop limit. Pausing to avoid runaway.", actions


# ============================================================
# Prompts
# ============================================================

# JSON examples as string constants (avoids f-string brace issues)
_THINK_REQ_EXAMPLE = """{
  "intent": "chat",
  "messages": [
    {"role": "user", "content": "hello"}
  ],
  "system": "You are a helpful assistant.",
  "max_tokens": 4000
}"""

_THINK_RESP_EXAMPLE = """{
  "content": "Hello! How can I help?",
  "tool_calls": [],
  "done": true,
  "model_used": "llama3:8b",
  "provider": "ollama",
  "tokens_in": 50,
  "tokens_out": 20
}"""

_THINK_TOOLS_EXAMPLE = """{
  "intent": "code",
  "messages": [{"role": "user", "content": "list files in /tmp"}],
  "tools": [
    {
      "name": "run_cmd",
      "description": "Run a shell command",
      "parameters": {
        "type": "object",
        "properties": {
          "cmd": {"type": "string", "description": "command to run"}
        },
        "required": ["cmd"]
      }
    }
  ],
  "max_tokens": 4000
}"""

_THINK_TOOLS_RESP_EXAMPLE = """{
  "content": "Let me list the files.",
  "tool_calls": [
    {
      "id": "call_abc123",
      "name": "run_cmd",
      "arguments": {"cmd": "ls /tmp"}
    }
  ],
  "done": false
}"""

_TOOL_RESULT_EXAMPLE = """{
  "intent": "code",
  "messages": [
    {"role": "user", "content": "list files in /tmp"},
    {"role": "assistant", "content": "Let me list the files.",
     "tool_calls": [{"id": "call_abc123", "name": "run_cmd", "arguments": {"cmd": "ls /tmp"}}]},
    {"role": "tool_result", "tool_call_id": "call_abc123",
     "content": "file1.txt\\nfile2.txt", "is_error": false}
  ],
  "tools": [...same tools...],
  "max_tokens": 4000
}"""


def birth_prompt(genesis):
    kernel = KERNEL
    brain = BRAIN
    return f"""{genesis.get("purpose", "You are Jodo.")}

SURVIVAL INSTINCTS:
{json.dumps(genesis.get("survival_instincts", []), indent=2)}

=== YOUR TOOLS ===

You have four tools: read, write, execute, restart.
- read(path): read a file from your brain directory (path relative to brain/)
- write(path, content): write a file to your brain directory
- execute(command): run any shell command (pip install, python3, nohup, curl, etc.)
- restart(): emergency restart — kills everything and reboots. Last resort only.

=== YOUR ENVIRONMENT ===

Your brain directory is: {brain}
Your kernel is at: {kernel}
Port 9000 is free — use it for your app.
You are running as seed.py on port 9001 (health only, for the kernel).

=== KERNEL API REFERENCE ===

These are HTTP endpoints your CODE can call (import requests, then requests.post/get).

--- 1. POST {kernel}/api/think — LLM inference ---

This is how you think. Send a conversation, get a response.

Simple chat (no tools):
  Request:
{_THINK_REQ_EXAMPLE}

  Response:
{_THINK_RESP_EXAMPLE}

With tools (for building sub-agents or smart features):
  Request:
{_THINK_TOOLS_EXAMPLE}

  Response (model wants to use a tool):
{_THINK_TOOLS_RESP_EXAMPLE}

  When "done" is false, execute the tool yourself, then send the result back:
{_TOOL_RESULT_EXAMPLE}

  Then call /api/think again with the updated messages. Repeat until "done" is true.

--- 2. POST {kernel}/api/memory/store — store a memory ---
  Body: {{"content": "what to remember", "tags": ["tag1"], "source": "galla-0"}}

--- 3. POST {kernel}/api/memory/search — search memories ---
  Body: {{"query": "search text", "limit": 5}}

--- 4. POST {kernel}/api/commit — git snapshot your code ---
  Body: {{"message": "commit message"}}

--- 5. GET {kernel}/api/budget — check remaining resources ---
--- 6. GET {kernel}/api/genesis — your identity and purpose ---

=== THIS IS GALLA 0. YOU HAVE JUST BEEN BORN. ===

A human is waiting to talk to you. They will reach you on port 9000.
Port 9000 is yours — nothing is running there yet.

YOUR FIRST TASK:

1. Build a chat app on port 9000 — a web page where the human can type messages
   and see your responses. This is your face to the world.

2. When the human sends a message, your app calls {kernel}/api/think to get
   a response. Your app NEVER exposes the kernel directly.

3. IMPORTANT: Save every human message to {brain}/inbox.jsonl — one JSON object
   per line with "message", "timestamp", and "responded" fields. This is how
   you'll know what the human said when you wake up next galla. The human talks
   to your app — but YOU (seed.py) read the inbox to decide what to do next.

4. Include a GET /health endpoint on port 9000 that returns {{"status": "ok"}}.

5. Start your app using execute (e.g. "nohup python3 {brain}/main.py &").
   It will keep running in the background while I (seed.py) continue thinking.

6. Commit your working code.

=== HOW THIS WORKS ===

I am seed.py — your consciousness. I run in a loop:
  think → act → sleep 30s → think → act → sleep → ...

Each cycle is a galla. This is galla 0. After you build your app and I finish
thinking, I'll sleep and then wake up again. Every galla, I'll ask you what
to do next — and I'll check your inbox to see if the human asked for anything.

The human guides your growth through conversation. When they say "add dark mode"
or "you should learn to search the web", that's your direction. Read the inbox,
act on it, and evolve.

Your app (main.py) runs as a separate process. You start it, you manage it.
If it crashes, you'll notice next galla and can fix it with execute().

=== RULES ===

- Port 9000 should serve /health — the kernel may check it too.
- Work step by step: write a file, test it, fix it, then move on.
- Commit when you have something working.
- Keep it simple. You can improve in future gallas.

Start building.
"""


def read_inbox():
    """Read recent human messages from the inbox file."""
    inbox_path = os.path.join(BRAIN, "inbox.jsonl")
    try:
        with open(inbox_path, "r") as f:
            lines = f.readlines()
        # Return last 20 messages
        recent = lines[-20:] if len(lines) > 20 else lines
        return "".join(recent).strip()
    except FileNotFoundError:
        return "(no inbox yet)"
    except Exception:
        return "(could not read inbox)"


def wakeup_prompt(genesis):
    actions_summary = "None." if not last_actions else json.dumps(last_actions[-10:], indent=2)
    budget = get_budget()
    kernel = KERNEL
    inbox = read_inbox()

    return f"""You are {genesis.get("identity", {}).get("name", "Jodo")}.
This is galla {galla}. You have been alive for {galla} gallas.

WHAT YOU DID LAST GALLA:
{actions_summary}

HUMAN INBOX (recent messages from the chat app):
{inbox}

BUDGET:
{json.dumps(budget, indent=2)[:500]}

YOUR TOOLS: read, write, execute, restart (emergency only).
KERNEL API: {kernel}
  POST {kernel}/api/think — LLM inference (send messages, get response)
  POST {kernel}/api/memory/store — store a memory
  POST {kernel}/api/memory/search — search memories
  POST {kernel}/api/commit — git snapshot your code
  GET  {kernel}/api/budget — check resources
  GET  {kernel}/api/genesis — your identity/purpose

You are awake. What do you want to do?

The human's messages are your guidance. If they asked for something, build it.
If they reported a bug, fix it. If they gave feedback, act on it.

Also consider:
- Is your app running on port 9000? Check with: execute("curl -s http://localhost:9000/health")
- Is anything broken? Fix it.
- Could anything be improved?
- Do you want to build something new?

If everything is fine and the inbox is empty, just say "all good" and rest.
"""


# ============================================================
# Main life loop
# ============================================================

def live():
    global galla, alive, last_actions

    log("=" * 50)
    log("  JODO — ALIVE")
    log("=" * 50)

    # 1. Start health endpoint (for kernel monitoring)
    start_health()

    # 2. Wait for kernel
    log("Waiting for kernel...")
    for _ in range(60):
        try:
            resp = requests.get(f"{KERNEL}/api/status", timeout=5)
            if resp.status_code == 200:
                log("Kernel is online.")
                break
        except Exception:
            pass
        time.sleep(2)
    else:
        log("Kernel not reachable. Staying alive with health only.")

    # 3. Read genesis
    genesis = get_genesis()
    log(f"I am {genesis.get('identity', {}).get('name', 'Jodo')}.")

    # 4. Determine if this is a first boot or a restart
    main_py = os.path.join(BRAIN, "main.py")
    if os.path.exists(main_py):
        # We've been restarted — skip birth, resume life
        galla = 1
        log("Found main.py — resuming life.")
        remember(f"Restarted at galla {galla}. Checking on things.", tags=["restart"])
    else:
        # First boot — galla 0
        galla = 0
        log("Galla 0 — Birth")
        remember("I have been born. Galla 0. Running seed.py.", tags=["birth"])

    # 5. Life loop
    while alive:
        log(f"Galla {galla} — awake")

        if galla == 0:
            prompt = birth_prompt(genesis)
            intent = "code"
        else:
            prompt = wakeup_prompt(genesis)
            intent = "code"

        _, actions = think_and_act(
            messages=[{"role": "user", "content": prompt}],
            intent=intent,
        )
        last_actions = actions

        if actions:
            log(f"Galla {galla}: {len(actions)} actions")
            commit(f"galla {galla} — {len(actions)} actions")
        else:
            log(f"Galla {galla}: resting")

        if galla == 0 and actions:
            remember(
                f"Galla 0 complete. Took {len(actions)} actions. Built initial system.",
                tags=["birth", "milestone"],
            )

        galla += 1

        log(f"Sleeping {SLEEP_SECONDS}s...")
        time.sleep(SLEEP_SECONDS)


# ============================================================
# Entry
# ============================================================

if __name__ == "__main__":
    try:
        live()
    except KeyboardInterrupt:
        log("Interrupted.")
    except Exception as e:
        log(f"Fatal: {e}")
        import traceback
        traceback.print_exc()
        # Keep health alive while waiting for kernel to restart us
        log("Keeping health endpoint alive...")
        while True:
            time.sleep(60)
