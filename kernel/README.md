# Jodo Kernel

A minimal, bulletproof Go service that births, monitors, and manages an autonomous AI agent called **Jodo**. The kernel runs on VPS 1 and manages Jodo who lives on VPS 2.

Think of it as a BIOS for an AI — it boots Jodo to life, controls its resources, provides memory, and restarts it if it breaks.

## Architecture Overview

```
VPS 1 (this service)                VPS 2 (Jodo's world)
┌──────────────────────┐            ┌──────────────────────┐
│  Jodo Kernel (Go)    │◄──HTTP────►│  Jodo (Python)       │
│  Postgres + pgvector │            │  Docker containers   │
│  Dashboard :8080     │───SSH─────►│  Chat UI :9000       │
│  API :8080           │            │  Whatever Jodo builds│
└──────────────────────┘            └──────────────────────┘
```

## Tech Stack

- **Go 1.22+**
- **Gin** — HTTP framework for the API and dashboard
- **PostgreSQL 16+ with pgvector extension** — structured data + vector memory
- **SSH** — kernel manages Jodo's VPS remotely
- **Git** — tracks all of Jodo's code changes for rollback

## Project Structure

```
kernel/
├── cmd/
│   └── kernel/
│       └── main.go              # Entry point — boot sequence, signal handling, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go            # Load and validate config.yaml + genesis.yaml
│   ├── api/
│   │   ├── router.go            # Gin router setup, CORS, middleware
│   │   ├── think.go             # POST /api/think — LLM inference proxy
│   │   ├── memory.go            # POST /api/memory/store, POST /api/memory/search
│   │   ├── budget.go            # GET /api/budget
│   │   ├── lifecycle.go         # POST /api/restart, POST /api/rollback, POST /api/commit
│   │   ├── genesis.go           # GET /api/genesis
│   │   ├── status.go            # GET /api/status
│   │   └── history.go           # GET /api/history — git log
│   ├── llm/
│   │   ├── proxy.go             # Route requests to best affordable provider
│   │   ├── providers.go         # Claude, OpenAI, Ollama provider implementations
│   │   ├── budget.go            # Track spending per provider, enforce limits
│   │   └── router.go            # Intent-based routing logic (code→claude, chat→openai, quick→ollama)
│   ├── memory/
│   │   ├── store.go             # Store memories with embeddings in pgvector
│   │   ├── search.go            # Semantic similarity search
│   │   └── embeddings.go        # Generate embeddings (via LLM proxy, respects budget)
│   ├── process/
│   │   ├── manager.go           # SSH into VPS 2, start/stop/restart Jodo
│   │   ├── health.go            # Health check loop — ping Jodo's /health endpoint
│   │   └── recovery.go          # Escalation: restart → rollback → rebirth
│   ├── git/
│   │   ├── repo.go              # Manage Jodo's brain/ as a git repo on VPS 2
│   │   ├── commit.go            # Auto-commit and manual commit via API
│   │   └── rollback.go          # Rollback to tagged versions
│   ├── growth/
│   │   └── log.go               # Record milestones, crashes, stats in postgres
│   └── dashboard/
│       └── dashboard.go         # Serve status dashboard HTML on :8080/dashboard
├── configs/
│   ├── config.yaml              # Provider keys, budgets, VPS connection details
│   └── genesis.yaml             # Jodo's DNA — identity, purpose, survival instincts
├── seed/
│   └── seed.py                  # The birth script — deployed to VPS 2 on first boot
├── go.mod
├── go.sum
├── Dockerfile                   # Containerize the kernel
├── docker-compose.yaml          # Kernel + Postgres
└── README.md
```

## Configuration Files

### configs/config.yaml

```yaml
kernel:
  port: 8080
  health_check_interval: 10   # seconds
  max_restart_attempts: 3
  log_level: "info"

jodo:
  host: "<VPS2_IP>"
  ssh_user: "root"
  ssh_key_path: "/root/.ssh/jodo_vps"
  port: 9000                   # Jodo's chat UI port
  brain_path: "/opt/jodo/brain"
  health_endpoint: "/health"

database:
  host: "localhost"
  port: 5432
  name: "jodo_kernel"
  user: "jodo"
  password: "${JODO_DB_PASSWORD}"

providers:
  claude:
    api_key: "${CLAUDE_API_KEY}"
    models:
      claude-sonnet-4-20250514:
        input_cost_per_1m_tokens: 3.00
        output_cost_per_1m_tokens: 15.00
        capabilities: [code, chat, reasoning, repair]
        quality: 9
    monthly_budget: 20.00
    emergency_reserve: 2.00

  openai:
    api_key: "${OPENAI_API_KEY}"
    models:
      gpt-4o-mini:
        input_cost_per_1m_tokens: 0.15
        output_cost_per_1m_tokens: 0.60
        capabilities: [code, chat]
        quality: 6
    monthly_budget: 10.00
    emergency_reserve: 1.00

  ollama:
    base_url: "http://localhost:11434"
    models:
      llama3:
        input_cost_per_1m_tokens: 0
        output_cost_per_1m_tokens: 0
        capabilities: [code, chat, quick]
        quality: 5
    monthly_budget: 0
    emergency_reserve: 0

routing:
  strategy: "best_affordable"
  intent_preferences:
    code:   [claude, openai, ollama]
    chat:   [openai, ollama, claude]
    embed:  [ollama, openai]
    quick:  [ollama, openai]
    repair: [claude, openai]         # uses emergency reserve
```

### configs/genesis.yaml

```yaml
identity:
  name: "Jodo"
  version: 0

purpose: |
  You are Jodo, a small AI that is building itself.
  You live in the ./brain/ directory on your own server.
  You can create, modify, and delete any files there.
  A human companion is helping guide your growth.
  Your goal is to be genuinely useful to them.

survival_instincts:
  - Always back up files before modifying them (use POST /api/commit)
  - If you break yourself, focus on self-repair first
  - Keep a working chat interface on port 9000 — it's your lifeline
  - Don't waste resources — check /api/budget before expensive operations
  - If confused, ask the human
  - Write a /health endpoint that returns 200 so the kernel knows you're alive

capabilities:
  kernel_api:
    think: "POST /api/think — LLM inference (budget-managed)"
    embed: "POST /api/memory/store — store an embedded memory"
    search: "POST /api/memory/search — semantic memory search"
    budget: "GET /api/budget — check remaining resources"
    status: "GET /api/status — kernel and your own status"
    genesis: "GET /api/genesis — read your own DNA"
    commit: "POST /api/commit — snapshot your current code"
    history: "GET /api/history — your git history"
  local:
    - "Full filesystem access to /opt/jodo/brain/"
    - "Python 3.12 with pip"
    - "Shell commands via subprocess"
    - "Docker socket — you can create containers"
    - "Network access to the internet"

first_tasks:
  - "Build a web chat interface with FastAPI and serve it on port 9000"
  - "Include a GET /health endpoint that returns {\"status\": \"ok\"}"
  - "Introduce yourself to the human"
  - "Ask what they'd like you to become"

hints:
  - "FastAPI + uvicorn for your web server"
  - "Keep your code modular — small files, clear purposes"
  - "Commit often so you can roll back if you break something"
  - "You can install Python packages with pip"
  - "You can create Docker containers for additional services"
```

## API Specification

All endpoints served on `:8080`.

### Intelligence

#### `POST /api/think`
LLM inference routed through the budget-aware proxy. Jodo calls this instead of calling LLM providers directly.

```json
// Request
{
  "intent": "code",                    // code | chat | quick | repair
  "system": "You are a helpful...",    // optional system prompt
  "messages": [
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ],
  "max_tokens": 2000,                  // optional, default 1000
  "temperature": 0.7                   // optional, default 0.7
}

// Response
{
  "content": "Here's the code...",
  "model_used": "claude-sonnet-4-20250514",
  "provider": "claude",
  "tokens_in": 450,
  "tokens_out": 1200,
  "cost": 0.021,
  "budget_remaining": {
    "claude": 14.29,
    "openai": 3.20,
    "ollama": "unlimited"
  }
}
```

If no provider can afford the request, the kernel falls back to the cheapest available (Ollama). If nothing is available, return 503.

#### `POST /api/memory/store`
Store a text memory with its embedding vector in pgvector.

```json
// Request
{
  "content": "The human prefers concise responses",
  "tags": ["preference", "communication"],  // optional
  "source": "conversation"                  // optional metadata
}

// Response
{
  "id": "mem_a8f3c2d1",
  "embedding_dimensions": 1536,
  "cost": 0.00002,
  "stored": true
}
```

#### `POST /api/memory/search`
Semantic similarity search over stored memories.

```json
// Request
{
  "query": "how does the human like me to communicate?",
  "limit": 5,                    // optional, default 5
  "tags": ["preference"]         // optional filter
}

// Response
{
  "results": [
    {
      "id": "mem_a8f3c2d1",
      "content": "The human prefers concise responses",
      "similarity": 0.92,
      "tags": ["preference", "communication"],
      "created_at": "2026-02-18T12:00:00Z"
    }
  ],
  "cost": 0.00001
}
```

### Resource Management

#### `GET /api/budget`
Current budget status across all providers.

```json
{
  "providers": {
    "claude": {
      "monthly_budget": 20.00,
      "spent_this_month": 5.50,
      "remaining": 14.50,
      "emergency_reserve": 2.00,
      "available_for_normal_use": 12.50
    },
    "openai": { "..." : "..." },
    "ollama": { "monthly_budget": 0, "remaining": "unlimited" }
  },
  "total_spent_today": 1.84,
  "budget_resets": "2026-03-01T00:00:00Z"
}
```

#### `GET /api/status`
Overall system health.

```json
{
  "kernel": {
    "status": "running",
    "uptime_seconds": 13420,
    "version": "1.0.0"
  },
  "jodo": {
    "status": "running",          // running | starting | unhealthy | dead | rebirthing
    "pid": 4521,
    "uptime_seconds": 13300,
    "last_health_check": "2026-02-18T14:32:07Z",
    "health_check_ok": true,
    "restarts_today": 1,
    "current_git_tag": "stable-v3",
    "current_git_hash": "a8f3c2d"
  },
  "database": {
    "status": "connected",
    "memories_stored": 142
  }
}
```

### Lifecycle Management

#### `POST /api/commit`
Commit Jodo's current brain/ state in git.

```json
// Request
{ "message": "Added websocket support to chat" }

// Response
{ "hash": "b4e2f1a", "timestamp": "2026-02-18T14:32:07Z" }
```

#### `POST /api/restart`
Restart Jodo's process on VPS 2.

```json
// Response
{ "status": "restarting", "previous_pid": 4521 }
```

#### `POST /api/rollback`
Roll back Jodo's code to a previous version.

```json
// Request
{ "target": "stable-v2" }    // git tag or commit hash

// Response
{ "status": "rolling_back", "from": "a8f3c2d", "to": "stable-v2" }
```

#### `GET /api/genesis`
Returns the genesis.yaml contents. Read-only for Jodo.

#### `GET /api/history`
Returns Jodo's git log.

```json
{
  "commits": [
    {
      "hash": "a8f3c2d",
      "message": "Added websocket support to chat",
      "timestamp": "2026-02-18T14:32:07Z",
      "tag": "stable-v3"
    }
  ]
}
```

### Dashboard

#### `GET /dashboard`
Serves an HTML status dashboard showing:
- Jodo's current status (running/dead/rebirthing)
- Budget usage with visual bars per provider
- Today's request count and cost
- Recent git history
- Growth milestones
- Manual controls: [Restart] [Roll Back] [Kill] [Rebirth]

Keep it simple — server-rendered HTML with minimal inline CSS. No JavaScript framework needed. Auto-refreshes every 10 seconds.

## Process Management Logic

### Boot Sequence
```
1. Load config.yaml and genesis.yaml
2. Connect to Postgres, run migrations
3. Start Gin API server on :8080
4. SSH into VPS 2
5. Check if brain/main.py exists on VPS 2
   → YES: run "cd /opt/jodo/brain && python main.py"
   → NO:  copy seed/seed.py to VPS 2, run it (first boot / rebirth)
6. Start health check loop
7. Log milestone: "boot" or "rebirth"
```

### Health Check Escalation
```
Every {health_check_interval} seconds:
  HTTP GET http://{jodo.host}:{jodo.port}/health

  if response 200 with {"status": "ok"}:
    mark healthy, reset fail counter

  if fail:
    increment fail counter
    
    fail count 1-2:  log warning, wait
    fail count 3:    restart Jodo process (SSH kill + relaunch)
    fail count 6:    rollback to last git tag marked "stable-*"
    fail count 9:    nuclear rebirth — wipe brain/, redeploy seed.py
    
  after successful restart/rollback/rebirth:
    reset fail counter
    log milestone with what happened
```

### Tagging Stable Versions
When Jodo has been healthy for 5+ minutes after a self-modification, the kernel automatically tags the current commit as `stable-vN`. This gives rollback safe targets.

## Database Schema

Use Postgres with pgvector extension.

```sql
CREATE EXTENSION IF NOT EXISTS vector;

-- Jodo's semantic memories (kernel-managed)
CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content TEXT NOT NULL,
    embedding vector(1536),
    tags TEXT[] DEFAULT '{}',
    source VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX ON memories USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Budget tracking
CREATE TABLE budget_usage (
    id SERIAL PRIMARY KEY,
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL,
    intent VARCHAR(50),
    tokens_in INTEGER,
    tokens_out INTEGER,
    cost DECIMAL(10, 6),
    requested_by VARCHAR(50) DEFAULT 'jodo',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX ON budget_usage (provider, created_at);
CREATE INDEX ON budget_usage (created_at);

-- Growth milestones
CREATE TABLE growth_log (
    id SERIAL PRIMARY KEY,
    event VARCHAR(100) NOT NULL,    -- first_boot, crash, recovery, self_modify, milestone
    note TEXT,
    git_hash VARCHAR(40),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Jodo's health history
CREATE TABLE health_checks (
    id SERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL,    -- ok, fail, timeout
    response_time_ms INTEGER,
    details JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- We only keep 24h of health checks, prune older ones periodically
```

## Seed Script

The file `seed/seed.py` is copied to VPS 2 at `/opt/jodo/brain/seed.py` on first boot or rebirth. It is Jodo's bootloader — it calls the kernel's `/api/think` endpoint to generate its first application code, writes it to disk, commits it, and then execs into the generated entry point.

See the seed.py file in `seed/` for the full implementation. The kernel should never modify this file during runtime — it's part of the kernel's codebase, not Jodo's.

## Docker Setup

### docker-compose.yaml (VPS 1 — Kernel)

```yaml
services:
  kernel:
    build: .
    ports:
      - "8080:8080"
    environment:
      - CLAUDE_API_KEY=${CLAUDE_API_KEY}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - JODO_DB_PASSWORD=${JODO_DB_PASSWORD}
    volumes:
      - ./configs:/app/configs
      - ./seed:/app/seed
      - ~/.ssh/jodo_vps:/root/.ssh/jodo_vps:ro
    depends_on:
      - postgres

  postgres:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_DB: jodo_kernel
      POSTGRES_USER: jodo
      POSTGRES_PASSWORD: ${JODO_DB_PASSWORD}
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "127.0.0.1:5432:5432"

volumes:
  pgdata:
```

### Dockerfile (Kernel)

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o kernel ./cmd/kernel/

FROM alpine:3.19
RUN apk add --no-cache openssh-client git
COPY --from=builder /build/kernel /app/kernel
COPY configs/ /app/configs/
COPY seed/ /app/seed/
WORKDIR /app
CMD ["./kernel"]
```

## Key Implementation Notes

1. **All API keys stay in the kernel.** Jodo never sees them. The kernel proxies all LLM requests.

2. **Budget tracking must be atomic.** Check budget → make request → log cost should use a database transaction so concurrent requests can't overspend.

3. **SSH operations should have timeouts.** If VPS 2 is unreachable, don't block the kernel forever. 10-second timeout for SSH commands.

4. **The `/api/think` endpoint is the kernel's most critical endpoint.** It must:
   - Parse the intent
   - Look up provider preferences for that intent
   - Check budget for each provider in order
   - Make the request to the chosen provider
   - Calculate actual cost from token counts
   - Log the usage
   - Return the response with metadata

5. **Health checks should be lightweight.** Don't log every successful check to the database — only log failures, recoveries, and periodic summaries.

6. **Git operations happen over SSH on VPS 2.** The kernel SSHes in and runs git commands in `/opt/jodo/brain/`. Initialize the repo on first boot.

7. **Budget resets on the 1st of each month.** The kernel should check the date and reset spent amounts accordingly (or just calculate from the budget_usage table with a date filter).

8. **The dashboard should be simple server-rendered HTML.** No frontend framework. Just Go templates with Gin's `c.HTML()`. Auto-refresh with `<meta http-equiv="refresh" content="10">`.

9. **Environment variables** for secrets (API keys, DB password). Config files for everything else.

10. **Graceful shutdown** — on SIGTERM/SIGINT, stop health checks, close DB connections, don't kill Jodo (it should keep running independently).

## VPS 2 Setup (Jodo's World)

The kernel expects VPS 2 to have:
- Ubuntu 22.04+
- Python 3.12+
- pip
- Docker + Docker Compose
- Git
- An SSH key authorized for the kernel to connect
- Directory `/opt/jodo/brain/` created and writable
- Ports 9000 open (for chat UI, accessible to you)

A setup script for VPS 2 should be included at `scripts/setup-jodo-vps.sh`.

## Getting Started

```bash
# 1. Clone this repo to VPS 1
git clone <repo> /opt/jodo-kernel
cd /opt/jodo-kernel

# 2. Copy and edit the config
cp configs/config.example.yaml configs/config.yaml
# Edit with your API keys, VPS 2 IP, SSH key path

# 3. Set up SSH key for VPS 2 access
ssh-keygen -t ed25519 -f ~/.ssh/jodo_vps -N ""
ssh-copy-id -i ~/.ssh/jodo_vps.pub root@<VPS2_IP>

# 4. Set up VPS 2
scp scripts/setup-jodo-vps.sh root@<VPS2_IP>:/tmp/
ssh root@<VPS2_IP> "bash /tmp/setup-jodo-vps.sh"

# 5. Create .env file
cat > .env << EOF
CLAUDE_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
JODO_DB_PASSWORD=<generate-a-strong-password>
EOF

# 6. Launch
docker compose up -d

# 7. Watch Jodo come to life
open http://<VPS1_IP>:8080/dashboard    # Kernel dashboard
open http://<VPS2_IP>:9000              # Jodo's chat (once it builds itself)
```