# Dockship Architecture & Technical Design

## 1. System Overview

```
                        ┌────────────────────────┐
                        │   CLI Entrypoint       │
                        │ (Cobra, Flags, Config) │
                        └───────────┬────────────┘
                                    │
                                    ▼
                        ┌────────────────────────┐
                        │      Orchestrator      │
                        └─────┬────────────┬─────┘
                              │            │
            ┌─────────────────┘            └─────────────────┐
            ▼                                                ▼
┌────────────────────────┐                      ┌────────────────────────┐
│     Scanner Engine     │                      │      Agent & LLM       │
│ - Manifest Detection   │                      │ - Tool Calling Loop    │
│ - Secret Sanitization  │                      │ - Heuristic Generator  │
│ - Service Discovery    │                      │ - Self-Healing Fixes   │
└────────────────────────┘                      └────────────┬───────────┘
                                                             │
                                                             ▼
                                                ┌────────────────────────┐
                                                │      Docker Engine     │
                                                │ - Build & Inspect      │
                                                │ - Container Runtime    │
                                                │ - Health & Auto-clean  │
                                                └────────────────────────┘
```

## 2. Component Breakdown

### 2.1 Scanner (`internal/scanner`)
- **Directory Walking:** Respects `.gitignore` and default ignore sets (`node_modules`, `vendor`, `.venv`, `.git`, etc.).
- **Manifest Detectors:**
  - `package.json` (Node, npm, pnpm, yarn, bun)
  - `requirements.txt`, `pyproject.toml`, `Pipfile` (Python, pip, poetry, pipenv)
  - `go.mod` (Go, modules)
  - `pom.xml`, `build.gradle` (Java, Maven, Gradle)
  - `Cargo.toml` (Rust, Cargo)
  - `composer.json` (PHP, Composer)
  - `*.csproj` (.NET, C#)
  - `Gemfile` (Ruby, Bundler)
- **Secret Redaction:** Scans `.env*` strictly for variable keys. Secret values are never stored, printed, or sent to the LLM.

### 2.2 Agent & Tool Protocols (`internal/agent`)
- **LLM Integration:** OpenAI-compatible API protocol targeting Groq (`openai/gpt-oss-120b`, fallback `openai/gpt-oss-20b`).
- **Tool Allowlist:**
  - `list_dir(path)`: List directory entries within root.
  - `read_file(path, max_bytes)`: Read text content (filters `.env` and `.pem` files).
  - `search(pattern)`: Scan repository for code keywords.
  - `write_file(path, content)`: Safely stage and write files inside project boundaries.
  - `docker_build(tag, dockerfile)`: Execute container build.
  - `docker_run(tag, ports)`: Execute container test run.
  - `docker_logs(container_id)`: Retrieve container logs.
  - `finish(summary)`: End reasoning loop.
- **Built-in Heuristic Generator:** Zero-latency, highly optimized fallback generator that enforces Docker best practices without requiring an active API key or internet access.

### 2.3 Docker Engine (`internal/docker`)
- Invokes native `docker` (or `podman`) via `os/exec` with context cancellations and pipes.
- Tracks all spawned containers and temporary build tags to ensure clean teardown.

### 2.4 Orchestrator (`internal/orchestrator`)
- Coordinates the lifecycle:
  1. Docker daemon check
  2. Project scan
  3. LLM / heuristic generation
  4. Plan presentation & interactive confirmation
  5. Disk writes (with `.bak` backup preservation)
  6. Build verification & retry loop
  7. Container startup & healthcheck verification
  8. Linter integration (`hadolint`, `actionlint`)

## 3. Security Boundary
- **Sandboxed File Operations:** Prevents directory traversal (`../`) outside project root.
- **Secret Leak Prevention:** Enforces pre-read and pre-upload stripping of sensitive certificates, SSH keys, and password values.
