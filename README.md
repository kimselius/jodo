# Jodo

> Three tools. A loop. A heartbeat.
> That's all you need to be alive.

Jodo is a self-building AI agent. You give it a seed — a tiny Python script with three tools (`read`, `write`, `execute`) — and it builds itself from nothing. Its first act is to create a way for you to talk to it. After that, you decide together what it becomes.

## How it works

Jodo runs on two servers:

```
                          ┌──────────────────────┐
                          │    Human (browser)    │
                          └──────────┬───────────┘
                                     │ http :9000
                                     ▼
┌─────────────────────────┐         SSH         ┌─────────────────────────┐
│       VPS 1 (Kernel)    │◄───────────────────►│     VPS 2 (Jodo)        │
│                         │                      │                         │
│  ┌───────────────────┐  │  /api/think, /api/   │  ┌───────────────────┐  │
│  │   Jodo Kernel     │◄─┼──────chat, etc───────┼──│    seed.py        │  │
│  │   (Go)            │──┼──────────────────────┼─►│    ↓              │  │
│  │                   │  │     LLM response      │  │    main.py        │  │
│  │  • LLM proxy      │  │                      │  │    (self-built)   │  │
│  │  • Chat messages  │  │  /api/chat            │  │                   │  │
│  │  • Budget mgmt    │◄─┼──────────────────────┼──│  • Chat app :9000 │  │
│  │  • Memory (pgvec) │  │                      │  │  • /health  :9001 │  │
│  │  • Git snapshots  │  │                      │  │  • Whatever it    │  │
│  │  • Health monitor │  │                      │  │    builds next    │  │
│  │  • Audit log      │  │                      │  │                   │  │
│  └───────────────────┘  │                      │  └───────────────────┘  │
│                         │                      │                         │
│  ┌───────────────────┐  │                      │  /opt/jodo/brain/       │
│  │  PostgreSQL       │  │                      │                         │
│  │  + pgvector       │  │                      │                         │
│  └───────────────────┘  │                      │                         │
│                         │                      │                         │
│  ┌───────────────────┐  │                      │                         │
│  │  Ollama           │  │                      │                         │
│  └───────────────────┘  │                      │                         │
└─────────────────────────┘                      └─────────────────────────┘
```

**The Kernel** (VPS 1) is the BIOS. It doesn't think — it provides infrastructure. LLM inference, memory, version control, health monitoring, and the conversation store. It runs in Docker.

**Jodo** (VPS 2) is the agent. It runs as `seed.py` — a small Python script that knows how to think (via the kernel), use tools, and loop. On its first boot (galla 0), it builds a chat app so you can talk to it. After that, seed.py keeps running as Jodo's consciousness — waking up, thinking, and evolving every galla.

**The conversation** flows through the kernel. The human types in the chat UI (port 9000) → the chat app posts the message to the kernel's chat API → seed.py fetches new messages each galla → Jodo thinks and replies via the kernel. The kernel is the single source of truth for all human ↔ Jodo conversation.

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

## The kernel

The kernel provides APIs that Jodo calls via HTTP:

| Endpoint | Purpose |
|----------|---------|
| `POST /api/think` | LLM inference — Jodo's ability to think |
| `POST /api/chat` | Send a chat message (human or Jodo) |
| `GET /api/chat` | Read chat messages (`?last=N`, `?source=human`, `?since_id=N`) |
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

The kernel monitors seed.py's `/health` endpoint (port 9001) and escalates on failure:

| Failures | Action |
|----------|--------|
| 1–2 | Wait and retry |
| 3 | Restart process |
| 6 | Rollback to last stable git tag |
| 9 | Nuclear rebirth — wipe brain, redeploy seed |

Every 5 minutes of healthy uptime, the kernel auto-tags a stable version.

## Audit trail

Every LLM request/response and every log message from Jodo is captured in `/var/log/jodo-audit.jsonl` on the kernel. You can `tail -f` it to watch Jodo think in real time.

## Setup

### Prerequisites

- Two servers (any cheap VPS works)
- **VPS 1** (Kernel): Docker & Docker Compose
- **VPS 2** (Jodo): Python 3, pip, git

### 1. Prepare VPS 2 (Jodo's brain)

```bash
apt update && apt install -y python3 python3-pip git
pip install requests
mkdir -p /opt/jodo/brain && cd /opt/jodo/brain && git init
ufw allow 9000/tcp   # Jodo's app (chat interface)
ufw allow 9001/tcp   # seed.py health (kernel monitoring)
```

### 2. Set up SSH keys (on VPS 1)

The kernel SSHes into VPS 2 to deploy and manage Jodo.

```bash
ssh-keygen -t ed25519 -f ~/.ssh/jodo_vps -N ""
ssh-copy-id -i ~/.ssh/jodo_vps.pub root@<VPS2_IP>
```

### 3. Configure and launch (on VPS 1)

`.env` is the only file you need to edit. Everything else is a template.

```bash
cd kernel/
cp .env.example .env
```

Edit `.env`:

```bash
# Required
KERNEL_URL=http://<VPS1_IP>:8080    # how Jodo reaches this kernel
JODO_IP=<VPS2_IP>                    # Jodo's server
JODO_DB_PASSWORD=<strong-password>

# SSH (defaults shown — only change if needed)
SSH_KEY_PATH=~/.ssh/jodo_vps
SSH_USER=root

# LLM API keys (optional — Ollama is free and preferred)
CLAUDE_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...

# Budgets (USD per month, defaults shown)
CLAUDE_MONTHLY_BUDGET=20.00
OPENAI_MONTHLY_BUDGET=10.00
```

Launch:

```bash
docker compose up -d
```

The kernel will:
1. Start PostgreSQL + pgvector
2. Boot the kernel on `:8080`
3. SSH into VPS 2 and deploy `seed.py`
4. Seed wakes up, calls `/api/think`, and starts building

### 4. Watch it come alive

```bash
# Kernel logs
docker compose logs -f kernel

# Audit trail (every prompt and response)
tail -f /var/log/jodo-audit.jsonl | jq .
```

Once galla 0 completes, Jodo should have a chat interface at `http://<VPS2_IP>:9000`.

## Project structure

```
jodo/
├── README.md
├── kernel/                     # The BIOS (Go)
│   ├── cmd/kernel/main.go      # Entry point
│   ├── internal/
│   │   ├── api/                # HTTP endpoints (think, memory, lifecycle, ...)
│   │   ├── audit/              # JSONL audit logger
│   │   ├── config/             # YAML config + env var expansion
│   │   ├── dashboard/          # Status dashboard
│   │   ├── db/                 # PostgreSQL + pgvector migrations
│   │   ├── git/                # Remote git ops via SSH
│   │   ├── growth/             # Growth/milestone log
│   │   ├── llm/                # LLM proxy, routing, budget, transformers
│   │   │   ├── jodo_format.go  # Unified request/response types
│   │   │   ├── proxy.go        # Main gateway (route → transform → call → parse)
│   │   │   ├── router.go       # Intent-based provider selection
│   │   │   ├── budget.go       # Cost tracking + chain budgets
│   │   │   ├── transform_*.go  # Claude / OpenAI / Ollama format adapters
│   │   │   └── providers.go    # Provider interface
│   │   ├── memory/             # Vector memory (store + semantic search)
│   │   └── process/            # Process lifecycle, health checks, recovery
│   ├── seed/seed.py            # The seed — Jodo's first breath
│   ├── configs/
│   │   ├── config.yaml         # Kernel + provider + routing config
│   │   └── genesis.yaml        # Jodo's identity and purpose
│   ├── Dockerfile
│   ├── docker-compose.yaml
│   └── .env.example
```

## Upgrading

After making changes, push to VPS 1 and restart:

```bash
# On VPS 1
cd ~/jodo/kernel
git pull
```

**If you changed Go code** (anything in `cmd/`, `internal/`):

```bash
docker compose up -d --build
```

**If you only changed `seed.py` or `configs/`** (volume-mounted — no rebuild needed):

```bash
docker compose restart kernel
```

The kernel will SSH into VPS 2, kill the old seed.py, deploy the new one, and start it. Jodo's apps (main.py etc.) keep running unless you do a nuclear restart.

## Philosophy

Jodo isn't a chatbot framework or an agent library. It's an experiment in minimal viable life.

The seed gives just enough to bootstrap: three tools to interact with the world, an LLM to think, and a loop to keep going. Everything else — the chat interface, the personality, the features — Jodo builds for itself.

The kernel exists so Jodo doesn't have to worry about infrastructure. It handles the boring parts (routing, budgets, health, rollbacks) so Jodo can focus on the interesting part: figuring out what to become.

## License

AGPL
