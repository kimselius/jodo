"""
seed.py — Jodo's Life

Three tools. A loop. A heartbeat.
That's all you need to be alive.
"""

import os
import json
import subprocess
import multiprocessing
import threading
import time
import requests
from http.server import HTTPServer, BaseHTTPRequestHandler
from dataclasses import dataclass, field

# ============================================================
# Config
# ============================================================

KERNEL = "__KERNEL_URL__"
BRAIN = "__BRAIN_PATH__"
HEALTH_PORT = __SEED_PORT__
SLEEP_SECONDS = int(os.environ.get("JODO_SLEEP_SECONDS", "30"))
MAX_SUBAGENTS = int(os.environ.get("JODO_MAX_SUBAGENTS", "__MAX_SUBAGENTS__"))
SUBAGENT_TIMEOUT = int(os.environ.get("JODO_SUBAGENT_TIMEOUT", "__SUBAGENT_TIMEOUT__"))

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
_phase = "booting"  # booting, thinking, sleeping
# Max time before health reports unhealthy (sleep + think timeout + buffer)
_HEARTBEAT_MAX = SLEEP_SECONDS + 600 + 60

# Subagent tracking
@dataclass
class AgentInfo:
    task_id: str
    prompt: str
    intent: str
    timeout: int
    process: multiprocessing.Process = None
    pid: int = 0
    start_time: float = 0.0
    status: str = "pending"  # pending, running, completed, failed, timed_out
    result: str = ""

_agents = {}           # task_id → AgentInfo
_agents_lock = threading.Lock()

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
            with _agents_lock:
                active = sum(1 for a in _agents.values() if a.status == "running")
            body = json.dumps({
                "status": status,
                "galla": galla,
                "alive": alive,
                "phase": _phase,
                "heartbeat_age": int(time.time() - _heartbeat),
                "active_agents": active,
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
    {
        "name": "spawn_agent",
        "description": "Spawn a subagent to work on a task in parallel. The subagent gets read, write, and execute tools (no restart, no spawning). It runs independently and posts results to your inbox when done. Use for execution-heavy tasks you want to delegate.",
        "parameters": {
            "type": "object",
            "properties": {
                "task_id": {
                    "type": "string",
                    "description": "Unique identifier for this task (e.g. 'fix-css', 'add-tests')",
                },
                "prompt": {
                    "type": "string",
                    "description": "Full instructions for the subagent. Be specific — it has no context beyond what you tell it.",
                },
                "intent": {
                    "type": "string",
                    "description": "LLM routing intent: 'code', 'chat', 'embed'. Default: 'code'",
                },
                "timeout": {
                    "type": "integer",
                    "description": f"Max seconds before the agent is killed. Default: {SUBAGENT_TIMEOUT}",
                },
            },
            "required": ["task_id", "prompt"],
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

# Read-only tools for the planning phase (no write, no restart)
PLAN_TOOLS = [t for t in TOOLS if t["name"] in ("read", "execute")]

TOOL_EXECUTORS = {
    "read": lambda args: _safe_tool("read", args, ["path"], lambda a: tool_read(a["path"])),
    "write": lambda args: _safe_tool("write", args, ["path", "content"], lambda a: tool_write(a["path"], a["content"])),
    "execute": lambda args: _safe_tool("execute", args, ["command"], lambda a: tool_execute(a["command"])),
    "restart": lambda args: tool_restart(),
    "spawn_agent": lambda args: tool_spawn_agent(args),
}

# Subagent tools — read, write, execute only (no restart, no spawn)
SUBAGENT_TOOLS = [t for t in TOOLS if t["name"] in ("read", "write", "execute")]

SUBAGENT_EXECUTORS = {
    "read": TOOL_EXECUTORS["read"],
    "write": TOOL_EXECUTORS["write"],
    "execute": TOOL_EXECUTORS["execute"],
}


# ============================================================
# Subagent system
# ============================================================

def subagent_runner(task_id, prompt, intent, timeout_secs):
    """Run a subagent in a child process. Communicates results via inbox POST."""
    import requests as req_lib
    session = req_lib.Session()
    session.proxies = {"http": None, "https": None}

    def agent_think(messages, system=None):
        payload = {
            "intent": intent,
            "messages": messages,
            "tools": SUBAGENT_TOOLS,
            "max_tokens": 8000,
            "requested_by": f"subagent:{task_id}",
        }
        if system:
            payload["system"] = system
        try:
            resp = session.post(f"{KERNEL}/api/think", json=payload, timeout=300)
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            return None

    def agent_log(msg):
        line = f"[subagent:{task_id}] {msg}"
        print(line, flush=True)
        try:
            session.post(
                f"{KERNEL}/api/log",
                json={"event": "subagent_log", "message": line, "galla": 0},
                timeout=2,
            )
        except Exception:
            pass

    system = f"You are a subagent working on task '{task_id}'. Complete the task and provide a summary of what you did."
    messages = [{"role": "user", "content": prompt}]
    actions = []
    max_loops = 30

    agent_log(f"started (intent={intent}, timeout={timeout_secs}s)")

    for i in range(max_loops):
        response = agent_think(messages, system=system)
        if response is None:
            agent_log("think call failed, aborting")
            break

        content = response.get("content", "")
        tool_calls = response.get("tool_calls", [])
        done = response.get("done", True)

        if not tool_calls or done:
            # Post result to main Jodo's inbox
            result_msg = content or "(no output)"
            try:
                session.post(
                    f"http://localhost:{HEALTH_PORT}/inbox",
                    json={"source": f"subagent:{task_id}", "message": result_msg[:2000]},
                    timeout=5,
                )
            except Exception:
                pass
            agent_log(f"completed after {i+1} iterations, {len(actions)} actions")
            return

        messages.append({
            "role": "assistant",
            "content": content,
            "tool_calls": tool_calls,
        })

        for tc in tool_calls:
            name = tc["name"]
            args = tc["arguments"]
            tc_id = tc["id"]

            agent_log(f"  tool: {name}({json.dumps(args)[:80]})")

            executor = SUBAGENT_EXECUTORS.get(name)
            if executor:
                result = executor(args)
                is_error = isinstance(result, str) and result.startswith("ERROR:")
            else:
                result = f"ERROR: Unknown tool: {name}"
                is_error = True

            actions.append({"tool": name, "args": args})

            messages.append({
                "role": "tool_result",
                "tool_call_id": tc_id,
                "content": result if isinstance(result, str) else str(result),
                "is_error": is_error,
            })

    # Hit loop limit
    try:
        session.post(
            f"http://localhost:{HEALTH_PORT}/inbox",
            json={"source": f"subagent:{task_id}", "message": f"Hit tool loop limit ({max_loops} iterations). Partial work done."},
            timeout=5,
        )
    except Exception:
        pass
    agent_log(f"hit loop limit after {len(actions)} actions")


def tool_spawn_agent(args):
    """Spawn a subagent to work on a task."""
    task_id = args.get("task_id", "")
    prompt = args.get("prompt", "")
    intent = args.get("intent", "code")
    timeout = args.get("timeout", SUBAGENT_TIMEOUT)

    if not task_id or not prompt:
        return "ERROR: task_id and prompt are required"

    with _agents_lock:
        # Check unique task_id
        if task_id in _agents and _agents[task_id].status == "running":
            return f"ERROR: agent '{task_id}' is already running"

        # Check concurrent limit
        running = sum(1 for a in _agents.values() if a.status == "running")
        if running >= MAX_SUBAGENTS:
            return f"ERROR: max concurrent subagents ({MAX_SUBAGENTS}) reached. Wait for one to finish."

        # Spawn process
        p = multiprocessing.Process(
            target=subagent_runner,
            args=(task_id, prompt, intent, timeout),
            daemon=True,
        )
        p.start()

        agent = AgentInfo(
            task_id=task_id,
            prompt=prompt[:200],
            intent=intent,
            timeout=timeout,
            process=p,
            pid=p.pid,
            start_time=time.time(),
            status="running",
        )
        _agents[task_id] = agent

    log(f"Spawned subagent '{task_id}' (PID {p.pid}, intent={intent}, timeout={timeout}s)")
    return f"OK: Subagent '{task_id}' spawned (PID {p.pid}). Results will appear in your inbox."


def check_agents():
    """Check on all tracked agents. Returns a status summary string."""
    lines = []
    with _agents_lock:
        for tid, agent in list(_agents.items()):
            if agent.status == "running":
                elapsed = time.time() - agent.start_time
                if not agent.process.is_alive():
                    # Process finished
                    agent.status = "completed"
                    agent.process.join(timeout=1)
                    lines.append(f"  [{tid}] completed (ran {int(elapsed)}s)")
                elif elapsed > agent.timeout:
                    # Timed out — kill it
                    agent.process.terminate()
                    agent.process.join(timeout=5)
                    agent.status = "timed_out"
                    log(f"Subagent '{tid}' timed out after {int(elapsed)}s — terminated")
                    lines.append(f"  [{tid}] TIMED OUT after {int(elapsed)}s")
                else:
                    lines.append(f"  [{tid}] running ({int(elapsed)}s / {agent.timeout}s)")
            elif agent.status in ("completed", "failed", "timed_out"):
                lines.append(f"  [{tid}] {agent.status}")

    if not lines:
        return "(no subagents)"
    return "\n".join(lines)


def cleanup_agents():
    """Terminate all running subagents."""
    with _agents_lock:
        for tid, agent in _agents.items():
            if agent.status == "running" and agent.process and agent.process.is_alive():
                agent.process.terminate()
                agent.process.join(timeout=5)
                agent.status = "failed"
                log(f"Cleaned up subagent '{tid}'")


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


def think(messages, system=None, intent="code", tools=None):
    """Call the kernel's /api/think endpoint."""
    payload = {
        "intent": intent,
        "messages": messages,
        "tools": tools if tools is not None else TOOLS,
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


def set_phase(phase):
    """Update phase locally and push to kernel for real-time UI."""
    global _phase
    _phase = phase
    with _agents_lock:
        active = sum(1 for a in _agents.values() if a.status == "running")
    try:
        kernel_http.post(
            f"{KERNEL}/api/heartbeat",
            json={"phase": phase, "galla": galla, "active_agents": active},
            timeout=2,
        )
    except Exception:
        pass


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


def post_galla(plan=None, summary=None, actions_count=None, galla_num=None):
    """Post galla plan/summary to kernel for the Growth timeline."""
    payload = {"galla": galla_num if galla_num is not None else galla}
    if plan is not None:
        payload["plan"] = plan
    if summary is not None:
        payload["summary"] = summary
    if actions_count is not None:
        payload["actions_count"] = actions_count
    try:
        kernel_http.post(f"{KERNEL}/api/galla", json=payload, timeout=10)
    except Exception as e:
        log(f"Galla post failed: {e}")


def ensure_galla_zero():
    """Backfill galla 0 if it's missing (for instances born before galla tracking)."""
    try:
        resp = kernel_http.get(f"{KERNEL}/api/galla?limit=1000", timeout=10)
        gallas_list = resp.json().get("gallas", [])
        if any(g["galla"] == 0 for g in gallas_list):
            return  # already exists
        log("Backfilling galla 0 record (born before tracking was added)")
        post_galla(
            plan="(birth — before galla tracking)",
            summary="Born and initialized.",
            actions_count=0,
            galla_num=0,
        )
    except Exception:
        pass


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
    """Fetch unread human chat messages from kernel."""
    try:
        resp = kernel_http.get(f"{KERNEL}/api/chat", params={"unread": "true", "source": "human"}, timeout=10)
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


def get_library():
    """Fetch library items from kernel."""
    try:
        resp = kernel_http.get(f"{KERNEL}/api/library", timeout=10)
        return resp.json().get("items", [])
    except Exception as e:
        log(f"Library fetch failed: {e}")
        return []


def update_library_status(item_id, status):
    """Update a library item's status."""
    try:
        kernel_http.patch(
            f"{KERNEL}/api/library/{item_id}",
            json={"status": status},
            timeout=10,
        )
    except Exception as e:
        log(f"Library status update failed: {e}")


def comment_library(item_id, message):
    """Add a comment to a library item."""
    try:
        kernel_http.post(
            f"{KERNEL}/api/library/{item_id}/comments",
            json={"source": "jodo", "message": message},
            timeout=10,
        )
    except Exception as e:
        log(f"Library comment failed: {e}")


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

def think_and_act(messages, system=None, intent="code", tools=None):
    """
    Send messages to the kernel. If the model wants to use tools,
    execute them and loop. Returns the final text response and
    all actions taken.
    """
    actions = []
    max_loops = 50

    for i in range(max_loops):
        response = think(messages, system=system, intent=intent, tools=tools)

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


PLAN_INSTRUCTIONS = """__PROMPT_PLAN__"""


def birth_prompt(genesis):
    return f"""__PROMPT_BIRTH__"""


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


def format_library(items):
    """Format library items for the wakeup prompt."""
    if not items:
        return "(no briefs)"
    lines = []
    for item in items:
        comments = f" ({len(item.get('comments', []))} comments)" if item.get("comments") else ""
        lines.append(f"  [{item['status']}] #{item['id']} {item['title']}{comments}")
    return "\n".join(lines)


def wakeup_prompt(genesis, inbox_messages, chat_messages):
    actions_summary = "None." if not last_actions else json.dumps(last_actions[-10:], indent=2)
    budget = get_budget()
    jodo_md = read_jodo_md()
    context = gather_context()
    library_items = get_library()
    library = format_library(library_items)
    subagent_status = check_agents()

    # Separate subagent results from system inbox
    agent_results = []
    system_msgs = []
    for m in inbox_messages:
        if m.get("source", "").startswith("subagent:"):
            agent_results.append(m)
        else:
            system_msgs.append(m)

    if system_msgs:
        inbox = "\n".join(f"[{m['source']}] {m['message']}" for m in system_msgs)
    else:
        inbox = "(no system messages)"

    if agent_results:
        inbox += "\n\nSUBAGENT RESULTS:\n" + "\n".join(
            f"[{m['source']}] {m['message']}" for m in agent_results
        )

    if chat_messages:
        chat = "\n".join(
            f"[{m.get('source', '?')}] {m.get('message', '')}" for m in chat_messages
        )
    else:
        chat = "(no new messages)"

    return f"""__PROMPT_WAKEUP__"""


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

    # 4b. Ensure galla 0 has a record (backfill for pre-tracking instances)
    if galla > 0:
        ensure_galla_zero()

    # 5. Life loop
    while alive:
        try:
            _heartbeat = time.time()
            set_phase("thinking")
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
                # Birth: no planning, just execute first tasks
                prompt = birth_prompt(genesis)
                post_galla(plan="(birth — no planning phase)")
                content, actions = think_and_act(
                    messages=[{"role": "user", "content": prompt}],
                    intent="code",
                )
                post_galla(summary=content or "", actions_count=len(actions))
            else:
                # Phase 1: Plan (read + execute only — inspect, then plan)
                set_phase("planning")
                prompt = wakeup_prompt(genesis, inbox_messages, chat_messages)
                plan, plan_actions = think_and_act(
                    messages=[{"role": "user", "content": prompt + "\n\n" + PLAN_INSTRUCTIONS}],
                    intent="plan",
                    tools=PLAN_TOOLS,
                )

                # Detect plan phase failures
                _PLAN_ERRORS = ["couldn't reach the kernel", "tool loop limit"]
                plan_failed = not plan or any(e in (plan or "").lower() for e in _PLAN_ERRORS)

                if plan_failed:
                    log(f"Plan phase failed: {(plan or '(empty)')[:100]}")
                    post_galla(plan=f"⚠ {plan}" if plan else "⚠ Planning failed")
                    # Give the execution phase a reasonable fallback
                    plan = "1. Check on human messages and respond if needed.\n2. Ensure my app is healthy.\n3. Make one small improvement."
                else:
                    log(f"Plan: {plan[:200]}")
                    post_galla(plan=plan)

                # Phase 2: Execute the plan (with tools)
                set_phase("thinking")
                exec_messages = [
                    {"role": "user", "content": prompt},
                    {"role": "assistant", "content": plan},
                    {"role": "user", "content": "Good plan. Now execute it. Use your tools."},
                ]
                content, actions = think_and_act(
                    messages=exec_messages,
                    intent="code",
                )
                post_galla(summary=content or "", actions_count=len(actions))

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
        set_phase("sleeping")

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
        cleanup_agents()
    except Exception as e:
        alive = False
        cleanup_agents()
        log(f"Fatal: {e}")
        import traceback
        traceback.print_exc()
        # Keep health endpoint running (reports unhealthy) so kernel can detect and restart us
        log("Life loop dead. Health endpoint reporting unhealthy...")
        while True:
            time.sleep(60)
