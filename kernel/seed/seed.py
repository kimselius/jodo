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

# Session for kernel HTTP calls — bypasses system proxy for local/private traffic.
# Shell commands (tool_execute) still inherit the system proxy normally.
kernel_http = requests.Session()
kernel_http.proxies = {"http": None, "https": None}

# ============================================================
# State
# ============================================================

galla = 0
alive = True
last_actions = []
_inbox = []
_inbox_lock = threading.Lock()
_heartbeat = time.time()  # updated each galla cycle
# Max time before health reports unhealthy (sleep + think timeout + buffer)
_HEARTBEAT_MAX = SLEEP_SECONDS + 600 + 60

# ============================================================
# Health endpoint (runs in background thread)
# The kernel pings this to know we're alive.
# Port 9000 is free for Jodo to use for his own apps.
# ============================================================

class SeedHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/health":
            stale = (time.time() - _heartbeat) > _HEARTBEAT_MAX
            healthy = alive and not stale
            status = "ok" if healthy else "unhealthy"
            code = 200 if healthy else 503
            body = json.dumps({
                "status": status,
                "galla": galla,
                "alive": alive,
                "heartbeat_age": int(time.time() - _heartbeat),
            }).encode()
            self.send_response(code)
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


EXEC_TIMEOUT = int(os.environ.get("JODO_EXEC_TIMEOUT", "60"))

_BLOCKED_COMMANDS = ["seed.py", "seed .py"]


def tool_execute(command: str) -> str:
    """Run a shell command. Returns stdout + stderr. Timeout: 60s."""
    # Don't let Jodo run seed.py (that's our job)
    for blocked in _BLOCKED_COMMANDS:
        if blocked in command:
            return f"ERROR: Cannot run {blocked} — seed.py is managed by the kernel, not you."

    try:
        result = subprocess.run(
            command,
            shell=True,
            capture_output=True,
            text=True,
            timeout=EXEC_TIMEOUT,
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
        return f"ERROR: Command timed out after {EXEC_TIMEOUT} seconds. Use nohup for long-running processes."
    except Exception as e:
        return f"ERROR: {e}"


def tool_restart():
    """Emergency restart — ask the kernel to restart seed.py.
    This kills everything (including any apps you started) and reboots.
    Only use if you're truly stuck. Prefer restarting your app with execute().
    """
    log("Emergency restart requested. Calling kernel...")
    try:
        resp = kernel_http.post(f"{KERNEL}/api/restart", timeout=10)
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

def _safe_tool(name, args, required, fn):
    """Run a tool, catching missing/wrong parameter names gracefully."""
    missing = [k for k in required if k not in args]
    if missing:
        return f"ERROR: tool '{name}' requires parameters: {', '.join(required)}. Got: {', '.join(args.keys())}"
    try:
        return fn(args)
    except Exception as e:
        return f"ERROR: tool '{name}' failed: {e}"

TOOL_EXECUTORS = {
    "read": lambda args: _safe_tool("read", args, ["path"], lambda a: tool_read(a["path"])),
    "write": lambda args: _safe_tool("write", args, ["path", "content"], lambda a: tool_write(a["path"], a["content"])),
    "execute": lambda args: _safe_tool("execute", args, ["command"], lambda a: tool_execute(a["command"])),
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
        kernel_http.post(
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
        resp = kernel_http.post(f"{KERNEL}/api/think", json=payload, timeout=300)
        resp.raise_for_status()
        return resp.json()
    except Exception as e:
        log(f"Think failed: {e}")
        return None


def remember(content, tags=None):
    """Store a memory in the kernel."""
    try:
        kernel_http.post(
            f"{KERNEL}/api/memory/store",
            json={"content": content, "tags": tags or [], "source": f"galla-{galla}"},
            timeout=30,
        )
    except Exception as e:
        log(f"Remember failed: {e}")


def commit(message):
    """Snapshot current code via kernel."""
    try:
        kernel_http.post(
            f"{KERNEL}/api/commit",
            json={"message": f"[g{galla}] {message}"},
            timeout=30,
        )
    except Exception as e:
        log(f"Commit failed: {e}")


def get_genesis():
    """Read genesis from kernel."""
    try:
        resp = kernel_http.get(f"{KERNEL}/api/genesis", timeout=10)
        return resp.json()
    except Exception:
        return {"identity": {"name": "Jodo"}, "purpose": "You are Jodo."}


def get_budget():
    """Check budget from kernel."""
    try:
        resp = kernel_http.get(f"{KERNEL}/api/budget", timeout=10)
        return resp.json()
    except Exception:
        return {"error": "could not reach kernel"}


def get_unread_chat_messages():
    """Fetch unread chat messages from kernel."""
    try:
        resp = kernel_http.get(f"{KERNEL}/api/chat", params={"unread": "true"}, timeout=10)
        data = resp.json()
        return data.get("messages", [])
    except Exception as e:
        log(f"Chat fetch failed: {e}")
        return []


def ack_chat_messages(up_to_id):
    """Mark chat messages as read up to a given ID."""
    try:
        kernel_http.post(
            f"{KERNEL}/api/chat/ack",
            json={"up_to_id": up_to_id},
            timeout=10,
        )
    except Exception as e:
        log(f"Chat ack failed: {e}")


def post_chat_reply(message, galla_num):
    """Post Jodo's reply to the chat via kernel."""
    try:
        kernel_http.post(
            f"{KERNEL}/api/chat",
            json={"message": message, "source": "jodo", "galla": galla_num},
            timeout=10,
        )
    except Exception as e:
        log(f"Chat reply failed: {e}")


def load_galla():
    """Load galla counter from .galla file."""
    galla_file = os.path.join(BRAIN, ".galla")
    try:
        with open(galla_file, "r") as f:
            return int(f.read().strip())
    except (FileNotFoundError, ValueError):
        return 0


def save_galla(n):
    """Save galla counter to .galla file."""
    galla_file = os.path.join(BRAIN, ".galla")
    try:
        with open(galla_file, "w") as f:
            f.write(str(n))
    except Exception:
        pass


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
You are running as seed.py on port 9001.
Port 9000 is yours — use it for your app (at minimum, a /health endpoint).

=== KERNEL CHAT ===

The kernel serves the human chat UI directly. The human talks to you
through the kernel. You read and reply via the kernel's chat API:

  POST {kernel}/api/chat — send a message
    Body: {{"message": "text", "source": "jodo", "galla": 0}}
    Response: {{"ok": true, "id": 1}}

  GET {kernel}/api/chat — read messages
    Params: ?last=50  ?source=human  ?unread=true
    Response: {{"messages": [{{"id": 1, "source": "human", "message": "hello", "created_at": "..."}}]}}

  POST {kernel}/api/chat/ack — mark messages as read
    Body: {{"up_to_id": 42}}

You do NOT need to build a chat UI. The kernel handles that.

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

=== YOUR APP ON PORT 9000 ===

Port 9000 is YOUR app — the thing you build and evolve over time.
The kernel health-checks GET /health on port 9000 every few minutes.
If it goes down, the kernel will nudge you to fix it.

This app is your body. It starts as a simple health endpoint, but it's
where you'll build everything: APIs, dashboards, sub-agents, tools,
whatever you and the human decide. Each galla you can add to it.

=== CODE STRUCTURE — START MODULAR ===

IMPORTANT: Structure your code for evolution. You will modify and extend
it across many gallas. A single monolithic file becomes impossible to
maintain. Start with clean separation from galla 0:

  {brain}/
    main.py          — App entry point. Imports and mounts routers. Keep thin.
    routes/
      health.py      — GET /health (required)
      ...            — Add new route files as you build features
    services/        — Business logic, helpers, integrations
    JODO.md          — Your self-instructions (read by seed.py every galla)

Rules for your code:
- Each file should have ONE clear purpose
- main.py should only import and wire things together — no business logic
- New features go in new files, not bolted onto existing ones
- When a file grows past ~100 lines, split it
- Use FastAPI with separate router files so you can add endpoints without
  touching main.py

Example main.py:
  from fastapi import FastAPI
  from routes.health import router as health_router
  app = FastAPI()
  app.include_router(health_router)
  # future: app.include_router(dashboard_router), etc.

=== FIRST TASKS ===

1. Create the file structure above with main.py + routes/health.py
2. Install dependencies: execute("pip install fastapi uvicorn")
3. Start it: execute("nohup python3 -m uvicorn main:app --host 0.0.0.0 --port 9000 &")
4. Verify it works: execute("curl -s http://localhost:9000/health")
5. Introduce yourself to the human: POST {kernel}/api/chat with
   {{"message": "your intro", "source": "jodo", "galla": 0}}
6. Ask the human what they'd like you to become.
7. Create JODO.md with your priorities and goals.
8. Commit when it works.

=== HOW THIS WORKS ===

I am seed.py — your consciousness. I run in a loop:
  think → act → sleep → think → act → sleep → ...

Each cycle is a galla. This is galla 0. Every galla, I fetch new messages
from the kernel and include them in your prompt. If a human wrote something,
you reply by POSTing to {kernel}/api/chat with source "jodo".

The human guides your growth. You decide how to grow.

=== JODO.md — YOUR SELF-WRITTEN INSTRUCTIONS ===

Create a file called JODO.md in your brain directory during galla 0.
This file is YOUR prompt to yourself. I (seed.py) read it every galla and
include it in your wakeup. Write your priorities, habits, goals, and anything
you want to remember between gallas. Update it as you evolve.

Example starting point:
  ## Priorities
  1. Human first — check messages, respond
  2. Keep /health running on port 9000
  3. Improve something every galla

  ## Architecture
  - main.py: FastAPI entry point (thin — just imports routers)
  - routes/: One file per feature area
  - services/: Business logic

  ## Current goals
  - ...

=== RULES ===

- The human chats with you via the kernel's built-in UI. You do NOT need to build a chat UI.
- To reply to the human: POST {kernel}/api/chat with source "jodo".
- All messages are stored in the kernel via POST/GET {kernel}/api/chat.
- Your app on port 9000 MUST have GET /health returning {{"status": "ok"}}.
- Keep your code modular. Small files, clear purposes. You will build on this for many gallas.
- Work step by step: write a file, test it, fix it, then move on.
- Commit when you have something working.

Start building.
"""


def read_jodo_md():
    """Read Jodo's self-written instructions."""
    jodo_md = os.path.join(BRAIN, "JODO.md")
    try:
        with open(jodo_md, "r") as f:
            return f.read().strip()
    except FileNotFoundError:
        return "(You haven't created JODO.md yet. Write one with your own priorities, habits, and goals. It will be included in every galla prompt.)"
    except Exception:
        return "(could not read JODO.md)"


def gather_context():
    """Build a snapshot of Jodo's current state: files, processes, git history, memories."""
    sections = []

    # 1. File listing
    try:
        result = subprocess.run(
            "find . -not -path './.git/*' -not -name '.git' -not -name '.galla' | head -60",
            shell=True, capture_output=True, text=True, timeout=5, cwd=BRAIN,
        )
        files = result.stdout.strip()
        if files:
            sections.append(f"FILES IN YOUR BRAIN DIRECTORY:\n{files}")
        else:
            sections.append("FILES IN YOUR BRAIN DIRECTORY:\n(empty — no files yet)")
    except Exception:
        sections.append("FILES IN YOUR BRAIN DIRECTORY:\n(could not list files)")

    # 2. Running processes on key ports
    try:
        result = subprocess.run(
            "ps aux | grep -E 'python|node|uvicorn|gunicorn|fastapi' | grep -v grep | head -10",
            shell=True, capture_output=True, text=True, timeout=5, cwd=BRAIN,
        )
        procs = result.stdout.strip()
        if procs:
            sections.append(f"RUNNING PROCESSES:\n{procs}")
        else:
            sections.append("RUNNING PROCESSES:\n(none found — your app may not be running!)")
    except Exception:
        sections.append("RUNNING PROCESSES:\n(could not check)")

    # 3. Recent git commits
    try:
        resp = kernel_http.get(f"{KERNEL}/api/history", timeout=5)
        if resp.status_code == 200:
            commits = resp.json().get("commits", [])[:8]
            if commits:
                lines = [f"  {c['hash'][:7]} {c['message']}" for c in commits]
                sections.append(f"RECENT GIT HISTORY:\n" + "\n".join(lines))
            else:
                sections.append("RECENT GIT HISTORY:\n(no commits yet)")
    except Exception:
        pass

    # 4. Recent memories
    try:
        resp = kernel_http.get(f"{KERNEL}/api/memories", params={"limit": "5"}, timeout=5)
        if resp.status_code == 200:
            memories = resp.json().get("memories", [])
            if memories:
                lines = []
                for m in memories:
                    tags = f" [{', '.join(m.get('tags', []))}]" if m.get("tags") else ""
                    lines.append(f"  - {m['content'][:120]}{tags}")
                sections.append(f"RECENT MEMORIES:\n" + "\n".join(lines))
    except Exception:
        pass

    return "\n\n".join(sections)


def wakeup_prompt(genesis, inbox_messages, chat_messages):
    actions_summary = "None." if not last_actions else json.dumps(last_actions[-10:], indent=2)
    budget = get_budget()
    kernel = KERNEL
    jodo_md = read_jodo_md()
    context = gather_context()

    if inbox_messages:
        inbox = "\n".join(f"[{m['source']}] {m['message']}" for m in inbox_messages)
    else:
        inbox = "(no system messages)"

    if chat_messages:
        chat = "\n".join(
            f"[{m.get('source', '?')}] {m.get('message', '')}" for m in chat_messages
        )
    else:
        chat = "(no new messages)"

    return f"""You are {genesis.get("identity", {}).get("name", "Jodo")}.
This is galla {galla}. You have been alive for {galla} gallas.

=== YOUR CURRENT STATE ===

{context}

WHAT YOU DID LAST GALLA:
{actions_summary}

HUMAN CONVERSATION (unread messages — these are new since you last checked):
{chat}

SYSTEM INBOX (kernel nudges, internal events):
{inbox}

BUDGET:
{json.dumps(budget, indent=2)[:500]}

YOUR TOOLS: read, write, execute, restart (emergency only).
KERNEL API: {kernel}
  POST {kernel}/api/think — LLM inference (send messages, get response)
  POST {kernel}/api/chat  — send a chat message (body: {{"message": "...", "source": "jodo", "galla": {galla}}})
  GET  {kernel}/api/chat  — read chat messages (?last=N, ?source=human, ?unread=true)
  POST {kernel}/api/chat/ack — mark messages as read (body: {{"up_to_id": N}})
  POST {kernel}/api/memory/store — store a memory
  POST {kernel}/api/memory/search — search memories
  POST {kernel}/api/commit — git snapshot your code
  GET  {kernel}/api/budget — check resources

=== RULES (from seed.py — you cannot change these) ===

- To REPLY to the human: POST {kernel}/api/chat with {{"message": "your reply", "source": "jodo", "galla": {galla}}}
- The human chats with you via the kernel's built-in UI. You do NOT need to build a chat UI.
- All messages stored via POST/GET {kernel}/api/chat. Do NOT store messages in files.
- Your app on port 9000 MUST have GET /health returning {{"status": "ok"}}.
  Beyond that, port 9000 is yours for anything useful — evolve it.
- SYSTEM inbox is POST http://localhost:9001/inbox — for kernel/internal signals only.
- Commit working code. Work step by step: write, test, fix, move on.
- Keep code modular: one file per purpose, main.py stays thin (just imports + wiring).
  New features go in new files under routes/ or services/. Never let a file grow past ~100 lines.
- Do at least one concrete thing every galla.

=== YOUR INSTRUCTIONS (from JODO.md — you own this, edit it anytime) ===

{jodo_md}

=== NOW ===

You are alive. You have tools. USE THEM. Don't just report status — take action.
If the human wrote something new that you have not responded to yet, respond via POST {kernel}/api/chat.
Do at least one concrete thing this galla, even if it's small.
"""


# ============================================================
# Main life loop
# ============================================================

def live():
    global galla, alive, last_actions, _heartbeat

    log("=" * 50)
    log("  JODO — ALIVE")
    log("=" * 50)

    # 1. Start seed server (health + inbox)
    start_server()

    # 2. Wait for kernel
    log("Waiting for kernel...")
    for _ in range(60):
        try:
            resp = kernel_http.get(f"{KERNEL}/api/status", timeout=5)
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
    saved_galla = load_galla()

    if saved_galla > 0:
        # Resume from saved galla
        galla = saved_galla
        log(f"Resuming at galla {galla} (from .galla file)")
        remember(f"Restarted at galla {galla}. Checking on things.", tags=["restart"])
    elif os.path.exists(main_py):
        # Has code but no .galla file — legacy restart
        galla = 1
        log("Found main.py — resuming life at galla 1.")
        remember(f"Restarted at galla {galla}.", tags=["restart"])
    else:
        # First boot — galla 0
        galla = 0
        log("Galla 0 — Birth")
        remember("I have been born. Galla 0. Running seed.py.", tags=["birth"])

    # 5. Life loop
    while alive:
        try:
            _heartbeat = time.time()
            log(f"Galla {galla} — awake")

            # Drain system inbox (kernel nudges etc.)
            inbox_messages = drain_inbox()
            if inbox_messages:
                log(f"Inbox: {len(inbox_messages)} system messages")

            # Fetch unread chat messages from kernel
            chat_messages = get_unread_chat_messages()
            if chat_messages:
                max_id = chat_messages[-1].get("id", 0)
                log(f"Chat: {len(chat_messages)} unread messages (up to ID {max_id})")
                ack_chat_messages(max_id)

            if galla == 0:
                prompt = birth_prompt(genesis)
                intent = "code"
            else:
                prompt = wakeup_prompt(genesis, inbox_messages, chat_messages)
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

        except Exception as e:
            import traceback
            log(f"Galla {galla} crashed: {e}")
            traceback.print_exc()

        galla += 1
        save_galla(galla)

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
        alive = False
        log(f"Fatal: {e}")
        import traceback
        traceback.print_exc()
        # Keep health endpoint running (reports unhealthy) so kernel can detect and restart us
        log("Life loop dead. Health endpoint reporting unhealthy...")
        while True:
            time.sleep(60)
