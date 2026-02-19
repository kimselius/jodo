# Jodo

> Three tools. A loop. A heartbeat.
> That's all you need to be alive.

Jodo is a self-building AI agent. You give it a seed — a tiny Python script with four tools (`read`, `write`, `execute`, `restart`) — and it builds itself from nothing. Its first act is to create a way for you to talk to it. After that, you decide together what it becomes.

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
│  │  • Health monitor  │  │                      │  │  • Whatever it    │  │
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

**The Kernel** is the BIOS. It doesn't think — it provides infrastructure. LLM inference, memory, version control, health monitoring, the chat UI, and the conversation store. It runs in Docker.

**Jodo** is the agent. It runs as `seed.py` — a small Python script that knows how to think (via the kernel), use tools, and loop. On its first boot (galla 0), it builds an app with a health endpoint so the kernel can monitor it. After that, seed.py keeps running as Jodo's consciousness — waking up, thinking, and evolving every galla.

**The conversation** flows through the kernel. The human types in the kernel's chat UI → seed.py fetches new messages each galla → Jodo thinks and replies via the kernel. The kernel is the single source of truth for all human-Jodo conversation.

Jodo can run as a **Docker container** on the same machine as the kernel (easiest), or on a **separate VPS** (more isolation). Both modes use SSH — the kernel SSHes into the Jodo environment to deploy, manage, and monitor.

## The seed

The seed gives Jodo four tools:

| Tool | What it does |
|------|-------------|
| `read(path)` | Read a file from its brain directory |
| `write(path, content)` | Write a file to its brain directory |
| `execute(command)` | Run any shell command |
| `restart()` | Emergency restart — kills everything and reboots seed.py |

And a life loop:

```
born → think → act → sleep → wake → think → act → sleep → ...
         ↑                                                 |
         └─────────────────────────────────────────────────┘
```

Each cycle is a **galla** — Jodo's unit of lived time.

Every wakeup galla follows a plan-then-execute pattern: seed.py first lets Jodo inspect its environment with read-only tools, then asks it to execute the plan with all tools available.

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
| `GET /api/genesis` | Read identity and purpose |
| `GET /api/status` | Kernel health status |

The kernel routes LLM requests to the cheapest capable provider:

1. **Ollama** (local, free) — preferred for everything
2. **Claude** — for complex reasoning and repair
3. **OpenAI** — fallback

Budget tracking prevents runaway costs. Each provider has a monthly cap with an emergency reserve.

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
4. **Providers** — Configure LLM providers (Ollama is enabled by default, optionally add Claude or OpenAI with API keys and budgets).
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
│   │   ├── audit/                 # JSONL audit logger
│   │   ├── config/                # DB-backed config store + encryption
│   │   ├── crypto/                # AES encryption for secrets
│   │   ├── db/                    # PostgreSQL + pgvector migrations
│   │   ├── git/                   # Remote git ops via SSH
│   │   ├── growth/                # Growth/milestone log
│   │   ├── llm/                   # LLM proxy, routing, budget
│   │   │   ├── proxy.go           # Main gateway (route → transform → call)
│   │   │   ├── router.go          # Intent-based provider selection
│   │   │   ├── budget.go          # Cost tracking per provider
│   │   │   └── transform_*.go     # Claude / OpenAI / Ollama adapters
│   │   ├── memory/                # Vector memory (store + semantic search)
│   │   └── process/               # Process lifecycle, health, recovery
│   ├── seed/
│   │   ├── seed.py                # The seed — Jodo's consciousness loop
│   │   └── prompts/               # birth, wakeup, plan prompt templates
│   ├── web/                       # Frontend (Vue 3 + TypeScript + Tailwind)
│   │   └── src/
│   │       ├── views/             # Chat, Status, Settings, Setup Wizard, ...
│   │       ├── components/        # UI components organized by feature
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

The seed gives just enough to bootstrap: four tools to interact with the world, an LLM to think, and a loop to keep going. Everything else — the personality, the features, the architecture — Jodo builds for itself.

The kernel exists so Jodo doesn't have to worry about infrastructure. It handles the boring parts (routing, budgets, health, rollbacks) so Jodo can focus on the interesting part: figuring out what to become.

## License

AGPL
