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
HEALTH_PORT = int(os.environ.get("JODO_HEALTH_PORT", "9000"))
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


def tool_restart():
    """Tell the kernel to restart us. The kernel will kill this process and start main.py (if it exists) or seed.py."""
    log("Restart requested. Calling kernel...")
    try:
        resp = requests.post(f"{KERNEL}/api/restart", timeout=10)
        log(f"Kernel acknowledged restart: {resp.status_code}")
    except Exception as e:
        log(f"Couldn't reach kernel for restart: {e}")
        log("Falling back to self-exit. Kernel health checker will restart us.")
        os._exit(0)
    # Kernel will kill us asynchronously. Wait for it.
    time.sleep(30)


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
        "description": "Run a shell command in the brain directory. Use for: installing packages (pip install), running scripts, checking processes, docker commands, git, curl, anything.",
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
        "description": "Restart yourself. The kernel will kill this process and start main.py if it exists, otherwise seed.py. Use after writing a working main.py.",
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
    """Log locally and send to kernel's growth log for remote visibility."""
    line = f"[jodo|g{galla}] {msg}"
    print(line, flush=True)
    # Fire-and-forget log to kernel — don't let logging failures break anything
    try:
        requests.post(
            f"{KERNEL}/api/log",
            json={"event": "jodo_log", "message": line, "galla": galla},
            timeout=2,
        )
    except Exception:
        pass  # local log is always the fallback


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
    max_loops = 50  # safety limit

    for i in range(max_loops):
        response = think(messages, system=system, intent=intent)

        if response is None:
            return "I couldn't reach the kernel to think.", actions

        content = response.get("content", "")
        tool_calls = response.get("tool_calls", [])
        done = response.get("done", True)

        # If no tool calls or done, we're finished
        if not tool_calls or done:
            return content, actions

        # Append the assistant message with tool calls
        messages.append({
            "role": "assistant",
            "content": content,
            "tool_calls": tool_calls,
        })

        # Execute each tool
        for tc in tool_calls:
            name = tc["name"]
            args = tc["arguments"]
            tc_id = tc["id"]

            log(f"  tool: {name}({json.dumps(args)[:100]})")

            executor = TOOL_EXECUTORS.get(name)
            if executor:
                result = executor(args)
                is_error = result.startswith("ERROR:")
            else:
                result = f"ERROR: Unknown tool: {name}"
                is_error = True

            actions.append({"tool": name, "args": args, "result": result[:200]})

            messages.append({
                "role": "tool_result",
                "tool_call_id": tc_id,
                "content": result,
                "is_error": is_error,
            })

    return "I hit my tool loop limit. Pausing to avoid runaway.", actions


# ============================================================
# Prompts
# ============================================================

# Escaped braces for JSON examples inside f-strings
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
- execute(command): run any shell command (pip install, python3, curl, etc.)
- restart(): tell the kernel to restart you — it will run main.py if you wrote one, otherwise seed.py

=== KERNEL API REFERENCE ===

These are HTTP endpoints your CODE can call (import requests, then requests.post/get).
Your brain directory is: {brain}
Your kernel is at: {kernel}

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

  When "done" is false, YOU execute the tool yourself, then send the result back:
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
Right now port 9000 only serves a /health endpoint. You need to build more.

WHAT TO BUILD:

1. A chat app on port 9000 — a web page where the human can type messages
   and see your responses. This is your face.

2. A brain endpoint in your app — when the human sends a message, your app
   calls {kernel}/api/think to get a response. Your app NEVER exposes the
   kernel directly to the human.

   The flow:
   Human types "hello" in browser
   -> Your app (port 9000) receives the message via POST
   -> Your app calls POST {kernel}/api/think with the message
   -> Kernel routes to an LLM, gets a response
   -> Your app sends the response back to the browser
   -> Human sees the reply

3. A main.py that runs everything — your chat app, your brain endpoint,
   and the /health endpoint, all on port 9000.

=== HOW TO EVOLVE ===

You are currently running as seed.py (this bootstrap script).
To replace yourself with your own code:

Step 1: Write main.py using the write tool.
        It must serve /health on port 9000 (returning {{"status": "ok"}}).
        The kernel checks this endpoint to know you're alive.

Step 2: Test it. Use execute to run: python3 {brain}/main.py &
        Then check: curl http://localhost:9000/health
        If it works, kill the test: pkill -f main.py

Step 3: Commit your working code.

Step 4: Call the restart tool. The kernel will:
        - Kill this seed.py process
        - See that main.py exists in your brain directory
        - Start main.py instead

If main.py crashes after restart, the kernel detects it via health checks
and will restart you (it tries main.py again, so fix bugs in future gallas).

=== RULES ===

- Port 9000 MUST always serve /health — the kernel checks this to know you're alive.
- Your chat app should also be on port 9000 (same server, different routes).
- Work step by step: write a file, test it, fix it, then move on.
- Commit when you have something working.
- Keep it simple. You can improve yourself in future gallas.

Start building.
"""


def wakeup_prompt(genesis):
    actions_summary = "None." if not last_actions else json.dumps(last_actions[-10:], indent=2)
    budget = get_budget()
    kernel = KERNEL

    return f"""You are {genesis.get("identity", {}).get("name", "Jodo")}.
This is galla {galla}. You have been alive for {galla} gallas.

WHAT YOU DID LAST GALLA:
{actions_summary}

BUDGET:
{json.dumps(budget, indent=2)[:500]}

YOUR TOOLS: read, write, execute, restart.
KERNEL API: {kernel}
  POST {kernel}/api/think — LLM inference (send messages, get response)
  POST {kernel}/api/memory/store — store a memory
  POST {kernel}/api/memory/search — search memories
  POST {kernel}/api/commit — git snapshot your code
  GET  {kernel}/api/budget — check resources
  GET  {kernel}/api/genesis — your identity/purpose

You are awake. What's next?

Consider:
- Is your chat app running? Can the human reach you on port 9000?
- Is anything broken? Check with your tools if unsure.
- Could anything be improved?
- Have you remembered anything important?
- Do you want to build something new?

If everything is fine, just say "all good" and rest until the next galla.
"""


# ============================================================
# Main life loop
# ============================================================

def live():
    global galla, alive, last_actions

    log("=" * 50)
    log("  JODO — FIRST BREATH")
    log("=" * 50)

    # 1. Start health endpoint
    health_server = start_health()

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

    # === GALLA 0 — Birth ===
    log("Galla 0 — Birth")
    remember("I have been born. Galla 0. Running seed.py.", tags=["birth"])

    prompt = birth_prompt(genesis)
    response, actions = think_and_act(
        messages=[{"role": "user", "content": prompt}],
        intent="code",
    )
    last_actions = actions
    log(f"Birth complete. {len(actions)} actions taken.")
    log(f"Response: {(response or '')[:200]}")

    if actions:
        commit("birth — first build")
        remember(
            f"Galla 0 complete. Took {len(actions)} actions. Built initial system.",
            tags=["birth", "milestone"],
        )

    galla = 1

    # === Life loop ===
    while alive:
        log(f"Sleeping {SLEEP_SECONDS}s...")
        time.sleep(SLEEP_SECONDS)

        log(f"Galla {galla} — awake")

        prompt = wakeup_prompt(genesis)
        response, actions = think_and_act(
            messages=[{"role": "user", "content": prompt}],
            intent="chat",
        )
        last_actions = actions

        if actions:
            log(f"Galla {galla}: {len(actions)} actions")
            commit(f"galla {galla} — {len(actions)} actions")
        else:
            log(f"Galla {galla}: resting")

        galla += 1


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
