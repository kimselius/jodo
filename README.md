# Jodo

> Five tools. A loop. A heartbeat.
> That's all you need to be alive.

Jodo is a self-building AI agent. You give it a seed — a tiny Python script with five tools (`read`, `write`, `execute`, `restart`, `spawn_agent`) — and it builds itself from nothing. Its first act is to create a way for you to talk to it. After that, you decide together what it becomes.

## How it works

```
                          ┌──────────────────────┐
                          │    Human (browser)    │
                          └──────────┬───────────┘
                                     │ http :8080
                                     ▼
┌─────────────────────────┐         SSH         ┌─────────────────────────┐
│       Kernel             │◄───────────────────►│     Jodo                │
│       (Go + Docker)      │                      │     (Docker or VPS)     │
│                          │  /api/think, /api/   │                         │
│  ┌────────────────────┐  │  ────chat, etc────►  │  ┌───────────────────┐  │
│  │   Jodo Kernel      │◄─┼──────────────────────┼──│    seed.py        │  │
│  │                    │──┼──────────────────────┼─►│    ↓              │  │
│  │  • Web UI + chat   │  │     LLM response      │  │    main.py        │  │
│  │  • LLM proxy       │  │                      │  │    (self-built)   │  │
│  │  • Budget mgmt     │  │                      │  │                   │  │
│  │  • Memory (pgvec)  │  │                      │  │  • /health  :9001 │  │
│  │  • Git snapshots   │  │                      │  │  • App      :9000 │  │
│  │  • Health monitor  │  │                      │  │  • Subagents      │  │
│  │  • Library + Inbox │  │                      │  │  • Whatever it    │  │
│  │  • Audit log       │  │                      │  │    builds next    │  │
│  │  • Setup wizard    │  │                      │  │                   │  │
│  └────────────────────┘  │                      │  └───────────────────┘  │
│                          │                      │                         │
│  ┌────────────────────┐  │                      │  /opt/jodo/brain/       │
│  │  PostgreSQL        │  │                      │                         │
│  │  + pgvector        │  │                      │                         │
│  └────────────────────┘  │                      │                         │
└──────────────────────────┘                      └─────────────────────────┘
```

**The Kernel** is the BIOS. It doesn't think — it provides infrastructure. LLM inference, memory, version control, health monitoring, the chat UI, the library, and the conversation store. It runs in Docker.

**Jodo** is the agent. It runs as `seed.py` — a small Python script that knows how to think (via the kernel), use tools, spawn subagents, and loop. On its first boot (galla 0), it builds an app with a health endpoint so the kernel can monitor it. After that, seed.py keeps running as Jodo's consciousness — waking up, thinking, delegating, and evolving every galla.

**The conversation** flows through the kernel. The human types in the kernel's chat UI → seed.py fetches new messages each galla → Jodo thinks and replies via the kernel. The kernel is the single source of truth for all human-Jodo conversation.

Jodo can run as a **Docker container** on the same machine as the kernel (easiest), or on a **separate VPS** (more isolation). Both modes use SSH — the kernel SSHes into the Jodo environment to deploy, manage, and monitor.

## The seed

The seed gives Jodo five tools:

| Tool | What it does |
|------|-------------|
| `read(path)` | Read a file from its brain directory |
| `write(path, content)` | Write a file to its brain directory |
| `execute(command)` | Run any shell command |
| `restart()` | Emergency restart — kills everything and reboots seed.py |
| `spawn_agent(task_id, prompt)` | Spawn a subagent to work on a task in parallel |

And a life loop:

```
born → think → act → sleep → wake → think → act → sleep → ...
         ↑                                                 |
         └─────────────────────────────────────────────────┘
```

Each cycle is a **galla** — Jodo's unit of lived time.

Every wakeup galla follows a plan-then-execute pattern: seed.py first lets Jodo inspect its environment with read-only tools, then asks it to execute the plan with all tools available.

### Subagents

Jodo can delegate work to subagents — child processes that run independently with their own `read`, `write`, and `execute` tools (but no `restart` or `spawn_agent` — no recursive spawning). Each subagent works on a single task, posts its result back to Jodo's inbox, and exits.

This lets Jodo act as a planner: it decides what to do, delegates execution-heavy work, and verifies the results next galla. Concurrency limits and timeouts are configurable.

## The kernel

The kernel provides APIs that Jodo calls via HTTP:

| Endpoint | Purpose |
|----------|---------|
| `POST /api/think` | LLM inference — Jodo's ability to think |
| `POST /api/chat` | Send a chat message (human or Jodo) |
| `GET /api/chat` | Read chat messages (`?last=N`, `?source=human`, `?unread=true`) |
| `POST /api/memory/store` | Store a memory (vector-embedded) |
| `POST /api/memory/search` | Semantic memory search |
| `POST /api/commit` | Git snapshot of Jodo's code |
| `POST /api/restart` | Restart Jodo's process |
| `GET /api/budget` | Check remaining LLM budget |
| `GET /api/budget/breakdown` | Per-model spending breakdown |
| `GET /api/genesis` | Read identity and purpose |
| `GET /api/status` | Kernel health status |
| `GET /api/library` | List library briefs (tasks from the human) |
| `PATCH /api/library/:id` | Update brief status |
| `POST /api/library/:id/comments` | Add a comment to a brief |
| `GET /api/inbox` | Read inbox messages (kernel/subagent comms) |

### LLM routing

The kernel routes LLM requests using `model@provider` references — each intent (code, chat, embed, etc.) has an ordered preference list of specific models:

```
code:  [glm4-flash@ollama, claude-sonnet-4@claude, gpt-4o@openai]
chat:  [glm4-flash@ollama, gpt-4o-mini@openai]
embed: [nomic-embed-text@ollama, text-embedding-3-small@openai]
```

The router tries each in order, skipping models that are over budget or busy (local models like Ollama have concurrency limits). Budget tracking prevents runaway costs — each provider has a monthly cap with an emergency reserve.

Model discovery is built in: the kernel can query Ollama for installed models and offers known catalogs for Claude and OpenAI with recommended defaults.

## The web UI

The kernel serves a full web UI for the human:

| Page | Purpose |
|------|---------|
| **Chat** | Talk to Jodo |
| **Status** | Kernel health, Jodo status, budget overview |
| **Library** | Create briefs (tasks/goals) for Jodo, track progress, comment threads |
| **Growth** | Galla log, milestones, events |
| **Logs** | Real-time log stream |
| **Inbox** | Read-only view of kernel-Jodo-subagent communications |
| **Memories** | Browse Jodo's vector memory |
| **Timeline** | Chronological view of all activity |
| **Settings** | Providers, routing, genesis, kernel, subagents, VPS |

The **Library** is how the human directs Jodo's work. Create a brief ("build a weather dashboard"), and Jodo picks it up next galla — updating status, adding comments as it works, and marking it done when finished.

The **Inbox** provides oversight of internal communications: kernel nudges, subagent results, system events. Read-only — the human observes but doesn't need to act.

## Health & recovery

The kernel monitors seed.py's `/health` endpoint and escalates on failure:

| Failures | Action |
|----------|--------|
| 1-2 | Wait and retry |
| 3 | Restart process |
| 6 | Rollback to last stable git tag |
| 9 | Nuclear rebirth — wipe brain, redeploy seed |

Every 5 minutes of healthy uptime, the kernel auto-tags a stable version. If Jodo's app (port 9000) goes down, the kernel nudges Jodo to fix it.

## Setup

### Prerequisites

- A server with **Docker** and **Docker Compose** installed
- Git (to clone this repo)

That's it. Everything else runs inside containers.

### Quick start

```bash
git clone <repo-url> jodo
cd jodo
chmod +x jodo.sh
./jodo.sh setup
./jodo.sh start
```

`setup` asks one question — where Jodo should live:

1. **Docker container** (default) — Jodo runs alongside the kernel on the same machine. Easiest to get started.
2. **Separate VPS** — you provide a second server. More isolation, but requires manual SSH key setup.

It then generates an encryption key, database password, and (in Docker mode) an SSH key pair. Everything is stored in `kernel/.env`.

### Setup wizard

After `./jodo.sh start`, open **http://your-server:8080** in your browser. The kernel starts in setup mode with a wizard that walks you through:

1. **SSH Connection** — In Docker mode, just verify the connection. In VPS mode, generate an SSH key, copy it to VPS 2, and verify.
2. **Server Setup** — Set the brain directory path and provision the server (creates the directory, initializes git, checks Python/pip).
3. **Kernel URL** (VPS mode only) — The URL Jodo uses to reach the kernel. In Docker mode this is auto-configured.
4. **Providers** — Configure LLM providers. Discover available Ollama models or pick from known Claude/OpenAI catalogs. Set budgets and enable the models you want.
5. **Genesis** — Jodo's identity: name, purpose, survival instincts, first tasks, and coding hints.
6. **Review & Birth** — Review everything and click "Birth Jodo".

All configuration is stored in the database (encrypted where sensitive). No config files to edit manually.

### Watch it come alive

```bash
./jodo.sh logs          # kernel logs
./jodo.sh logs jodo     # Jodo container logs (Docker mode)
```

Once galla 0 completes, Jodo should have a `/health` endpoint running on port 9000 and have introduced itself in the chat UI at **http://your-server:8080**.

### VPS mode

If you chose VPS mode, you need a second server with Python 3, pip, and git. The setup wizard handles SSH key generation and connection verification. You'll need to manually copy the public key to VPS 2's `~/.ssh/authorized_keys`.

```bash
# On VPS 2
ufw allow 9000/tcp   # Jodo's app
ufw allow 9001/tcp   # seed.py health endpoint
```

## Managing Jodo

```bash
./jodo.sh setup      # Generate .env and configure deployment mode
./jodo.sh start      # Start all containers
./jodo.sh stop       # Stop all containers
./jodo.sh logs       # Follow kernel logs (or: logs jodo, logs postgres)
./jodo.sh destroy    # Stop and delete all data (requires typing 'destroy')
./jodo.sh help       # Show available commands
```

## Audit trail

Every LLM request/response and every log message from Jodo is captured in `/var/log/jodo-audit.jsonl` on the kernel. You can `tail -f` it to watch Jodo think in real time.

## Project structure

```
jodo/
├── jodo.sh                        # CLI for setup, start, stop, destroy
├── kernel/                        # The BIOS (Go + Vue)
│   ├── cmd/kernel/main.go         # Entry point
│   ├── internal/
│   │   ├── api/                   # HTTP endpoints + setup wizard
│   │   │   ├── router.go          # Route registration + CORS
│   │   │   ├── settings.go        # Provider config, model discovery
│   │   │   ├── library.go         # Library CRUD + comments
│   │   │   ├── inbox.go           # Inbox message log
│   │   │   └── budget.go          # Budget status + per-model breakdown
│   │   ├── audit/                 # JSONL audit logger
│   │   ├── config/                # DB-backed config store + encryption
│   │   ├── crypto/                # AES encryption for secrets
│   │   ├── db/                    # PostgreSQL + pgvector migrations
│   │   ├── git/                   # Remote git ops via SSH
│   │   ├── growth/                # Growth/milestone log
│   │   ├── llm/                   # LLM proxy, routing, budget
│   │   │   ├── proxy.go           # Main gateway (route → transform → call)
│   │   │   ├── router.go          # model@provider routing with fallback
│   │   │   ├── budget.go          # Cost tracking per provider
│   │   │   ├── busy.go            # Concurrency tracking for local models
│   │   │   └── transform_*.go     # Claude / OpenAI / Ollama adapters
│   │   ├── memory/                # Vector memory (store + semantic search)
│   │   └── process/               # Process lifecycle, health, recovery
│   ├── seed/
│   │   ├── seed.py                # The seed — Jodo's consciousness loop
│   │   └── prompts/               # birth, wakeup, plan prompt templates
│   ├── web/                       # Frontend (Vue 3 + TypeScript + Tailwind)
│   │   └── src/
│   │       ├── views/             # Chat, Status, Library, Growth, Logs,
│   │       │                      # Inbox, Memories, Timeline, Settings
│   │       ├── components/        # UI components organized by feature
│   │       │   ├── layout/        # Sidebar, navigation
│   │       │   ├── status/        # BudgetCard, StatusCards
│   │       │   ├── library/       # LibraryCard, LibraryForm, CommentThread
│   │       │   ├── settings/      # ProvidersTab, RoutingTab, ModelDiscovery
│   │       │   └── ui/            # Button, Card, Input, Badge, etc.
│   │       ├── composables/       # Vue composition functions
│   │       └── lib/               # API client, SSE, utilities
│   ├── docker/
│   │   └── jodo/                  # Jodo container (Python + SSH)
│   │       ├── Dockerfile
│   │       └── entrypoint.sh
│   ├── Dockerfile                 # Kernel container (Go + Alpine)
│   └── docker-compose.yaml
```

## Upgrading

```bash
cd ~/jodo
git pull
./jodo.sh start    # rebuilds containers automatically
```

The kernel will SSH into Jodo's environment, deploy the updated seed.py, and restart. Jodo's self-built apps (main.py etc.) keep running unless a nuclear restart occurs.

## Philosophy

Jodo isn't a chatbot framework or an agent library. It's an experiment in minimal viable life.

The seed gives just enough to bootstrap: five tools to interact with the world, an LLM to think, and a loop to keep going. Everything else — the personality, the features, the architecture — Jodo builds for itself.

The kernel exists so Jodo doesn't have to worry about infrastructure. It handles the boring parts (routing, budgets, health, rollbacks) so Jodo can focus on the interesting part: figuring out what to become.

The human guides through conversation and library briefs. Jodo plans, delegates to subagents, and evolves. Each galla it gets a little better at whatever it's becoming.

## License

AGPL
