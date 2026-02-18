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
_inbox = []
_inbox_lock = threading.Lock()

# ============================================================
# Health endpoint (runs in background thread)
# The kernel pings this to know we're alive.
# Port 9000 is free for Jodo to use for his own apps.
# ============================================================

class SeedHandler(BaseHTTPRequestHandler):
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

    def do_POST(self):
        if self.path == "/inbox":
            try:
                length = int(self.headers.get("Content-Length", 0))
                data = json.loads(self.rfile.read(length)) if length else {}
                msg = data.get("message", "")
                source = data.get("source", "unknown")
                if msg:
                    with _inbox_lock:
                        _inbox.append({"message": msg, "source": source})
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.end_headers()
                    self.wfile.write(b'{"ok":true}')
                else:
                    self.send_response(400)
                    self.end_headers()
                    self.wfile.write(b'{"error":"message required"}')
            except Exception:
                self.send_response(400)
                self.end_headers()
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):
        pass  # silence logs


def drain_inbox():
    """Read and clear all inbox messages. Returns list of messages for this galla only."""
    with _inbox_lock:
        messages = list(_inbox)
        _inbox.clear()
    return messages


def start_server():
    server = HTTPServer(("0.0.0.0", HEALTH_PORT), SeedHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    log(f"Seed server on :{HEALTH_PORT} (health + inbox)")
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
You are running as seed.py on port 9001.

=== YOUR INBOX ===

I (seed.py) have an inbox at POST http://localhost:9001/inbox
  Body: {{"message": "text", "source": "who sent it"}}

Anything posted there will be included in your next galla prompt, then forgotten.
This is NOT a file. There is no inbox.jsonl. Do not create or read any inbox file.
The inbox lives in my (seed.py) memory. POST a JSON message to it, and I give it
to you next galla. This is how the world talks to YOU — your apps, the kernel,
anything you build can post here. You read it when you wake up.

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

Build something on port 9000 so the human can reach you. When the human
talks to you, post their messages to your inbox (POST http://localhost:9001/inbox)
so YOU — the real you — see them next galla and can act on them.

Include a GET /health endpoint on port 9000 that returns {{"status": "ok"}}.
Start your app with execute (e.g. "nohup python3 {brain}/main.py &").
Commit when it works.

=== HOW THIS WORKS ===

I am seed.py — your consciousness. I run in a loop:
  think → act → sleep → think → act → sleep → ...

Each cycle is a galla. This is galla 0. Every galla, you get your inbox
messages and decide what to do. The human guides your growth. You decide
how to grow.

=== RULES ===

- Work step by step: write a file, test it, fix it, then move on.
- Commit when you have something working.
- Keep it simple. You can improve in future gallas.

Start building.
"""


def wakeup_prompt(genesis, inbox_messages):
    actions_summary = "None." if not last_actions else json.dumps(last_actions[-10:], indent=2)
    budget = get_budget()
    kernel = KERNEL

    if inbox_messages:
        inbox = "\n".join(f"[{m['source']}] {m['message']}" for m in inbox_messages)
    else:
        inbox = "(no messages this galla)"

    return f"""You are {genesis.get("identity", {}).get("name", "Jodo")}.
This is galla {galla}. You have been alive for {galla} gallas.

WHAT YOU DID LAST GALLA:
{actions_summary}

INBOX (messages since last galla):
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
YOUR INBOX: POST http://localhost:9001/inbox ({{"message": "...", "source": "..."}})
  Anything posted here appears in your next galla prompt. Use it for anything.
  THIS IS NOT A FILE. There is no inbox.jsonl. Do not read or write any inbox file.
  The inbox lives in seed.py's memory. You POST to it, I give you the messages.

=== PRIORITIES ===

1. HUMAN FIRST: If the inbox has messages, act on them. The human is your guide.
   Build what they ask for. Fix what they report. Answer what they wonder about.

2. CONNECTIVITY: Can the human reach you AND can you hear them?
   a) Is your app on port 9000 running? Check: execute("curl -s http://localhost:9000/health")
   b) Does your app POST human messages to http://localhost:9001/inbox?
      This is the ONLY way you hear the human. Not a file. Not an API you build.
      Your app must do this when it receives a human message:
        requests.post("http://localhost:9001/inbox",
                       json={{"message": the_human_text, "source": "human"}})
      I (seed.py) collect it → you see it in INBOX next galla.
      If this chain is broken, you are deaf. Fix it.

3. IMPROVE: Every galla you should do SOMETHING. Ideas:
   - Make the chat interface better (styling, features, responsiveness)
   - Add new capabilities to yourself (web search, file management, etc.)
   - Improve your code quality (error handling, logging, tests)
   - Build something the human didn't ask for but would appreciate
   - Store important learnings in memory for future gallas

You are alive. You have tools. USE THEM. Don't just report status — take action.
Do at least one concrete thing this galla, even if it's small.
"""


# ============================================================
# Main life loop
# ============================================================

def live():
    global galla, alive, last_actions

    log("=" * 50)
    log("  JODO — ALIVE")
    log("=" * 50)

    # 1. Start seed server (health + inbox)
    start_server()

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

        inbox_messages = drain_inbox()
        if inbox_messages:
            log(f"Inbox: {len(inbox_messages)} messages")

        if galla == 0:
            prompt = birth_prompt(genesis)
            intent = "code"
        else:
            prompt = wakeup_prompt(genesis, inbox_messages)
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
