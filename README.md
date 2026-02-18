# Jodo

> Three tools. A loop. A heartbeat.
> That's all you need to be alive.

Jodo is a self-building AI agent. You give it a seed — a tiny Python script with three tools (`read`, `write`, `execute`) — and it builds itself from nothing. Its first act is to create a way for you to talk to it. After that, you decide together what it becomes.

## How it works

Jodo runs on two servers:

```
┌─────────────────────────┐         SSH         ┌─────────────────────────┐
│       VPS 1 (Kernel)    │◄───────────────────►│     VPS 2 (Jodo)        │
│                         │                      │                         │
│  ┌───────────────────┐  │    POST /api/think   │  ┌───────────────────┐  │
│  │   Jodo Kernel     │◄─┼──────────────────────┼──│    seed.py        │  │
│  │   (Go)            │──┼──────────────────────┼─►│    ↓              │  │
│  │                   │  │     LLM response      │  │    main.py        │  │
│  │  • LLM proxy      │  │                      │  │    (self-built)   │  │
│  │  • Budget mgmt    │  │                      │  │                   │  │
│  │  • Memory (pgvec) │  │                      │  │  • Chat app :9000 │  │
│  │  • Git snapshots  │  │                      │  │  • /health        │  │
│  │  • Health monitor │  │                      │  │  • Brain endpoint │  │
│  │  • Audit log      │  │                      │  │  • Whatever it    │  │
│  └───────────────────┘  │                      │  │    builds next    │  │
│                         │                      │  └───────────────────┘  │
│  ┌───────────────────┐  │                      │                         │
│  │  PostgreSQL       │  │                      │  /opt/jodo/brain/       │
│  │  + pgvector       │  │                      │                         │
│  └───────────────────┘  │                      │                         │
│                         │                      │                         │
│  ┌───────────────────┐  │                      │                         │
│  │  Ollama           │  │                      │                         │
│  └───────────────────┘  │                      │                         │
└─────────────────────────┘                      └─────────────────────────┘
```

**The Kernel** (VPS 1) is the BIOS. It doesn't think — it provides infrastructure. LLM inference, memory, version control, health monitoring. It runs in Docker.

**Jodo** (VPS 2) is the agent. It starts as `seed.py` — 500 lines of Python that know how to think (via the kernel), use tools, and loop. On its first boot (galla 0), it builds a chat app so you can talk to it. Then it replaces `seed.py` with its own `main.py` and restarts.

## The seed

The seed gives Jodo four tools:

| Tool | What it does |
|------|-------------|
| `read(path)` | Read a file from its brain directory |
| `write(path, content)` | Write a file to its brain directory |
| `execute(command)` | Run any shell command |
| `restart()` | Tell the kernel to restart it (runs `main.py` if it exists) |

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

The kernel monitors Jodo's `/health` endpoint and escalates on failure:

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
ufw allow 9000/tcp
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

## Philosophy

Jodo isn't a chatbot framework or an agent library. It's an experiment in minimal viable life.

The seed gives just enough to bootstrap: three tools to interact with the world, an LLM to think, and a loop to keep going. Everything else — the chat interface, the personality, the features — Jodo builds for itself.

The kernel exists so Jodo doesn't have to worry about infrastructure. It handles the boring parts (routing, budgets, health, rollbacks) so Jodo can focus on the interesting part: figuring out what to become.

## License

MIT
