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
│ - Multi-Stack Analysis │                      │ - Tool Calling Loop    │
│ - Secret Redaction     │                      │ - Heuristic Generator  │
│ - Extension & Web      │                      │ - Resilient 429 Retry  │
└────────────────────────┘                      └────────────┬───────────┘
                                                             │
                                                             ▼
                                                ┌────────────────────────┐
                                                │      Docker Engine     │
                                                │ - Build & Inspect      │
                                                │ - Safe Port Mapping    │
                                                │ - Health & Auto-clean  │
                                                └────────────────────────┘
```

## 2. Component Breakdown

### 2.1 Scanner Engine (`internal/scanner`)
- **Directory Walking:** Respects `.gitignore` and default ignore sets (`node_modules`, `vendor`, `.venv`, `.git`, `.gotmp`, `bin`, etc.).
- **Multi-Stack Detectors:**
  - `manifest.json` (Chrome Extensions / WebExtensions)
  - `index.html` (Static Web, HTML/CSS/JS)
  - `package.json` (Node.js, npm, pnpm, yarn, bun)
  - `requirements.txt`, `pyproject.toml`, `Pipfile` (Python, pip, poetry, pipenv)
  - `go.mod` (Go, modules, cmd entrypoint discovery)
  - `pom.xml`, `build.gradle` (Java, Maven, Gradle)
  - `Cargo.toml` (Rust, Cargo)
  - `composer.json` (PHP, Composer)
  - `*.csproj` (.NET, C#)
  - `Gemfile` (Ruby, Bundler)
- **Scoping & Secret Redaction:** Environment variables are strictly parsed from the root project directory, preventing nested `examples/` from polluting root dependencies. Variable values are completely stripped.

### 2.2 Agent & Resilient Heuristics Engine (`internal/agent`)
- **LLM Integration:** OpenAI-compatible API protocol targeting Groq (`openai/gpt-oss-120b`, fallback `openai/gpt-oss-20b`).
- **Tolerant Tool Protocol:** Parameters support multi-aliases (`pattern`/`query`/`keyword`, `path`/`file`, `content`/`code`) to ensure 100% schema compliance across varied LLM outputs.
- **Fail-Safe Graceful Fallback:** If Groq encounters rate limits (429 TPM exhaustion) or downtime, Dockship does NOT crash. It seamlessly transitions to the built-in Expert Heuristics Engine, generating production-grade artifacts and continuing Docker validation without interruption.

### 2.3 Docker Engine (`internal/docker`)
- Invokes native `docker` (or `podman`) via `os/exec` with context cancellations and pipes.
- **Safe Port Binding:** Ports `<= 0` are never bound (preventing daemon errors like `invalid port specification: "0/tcp"`).
- **Exit Code Verification:** CLI containers that terminate with exit code `0` (e.g. `--help` check) are correctly verified as healthy.
- Tracks all spawned containers and temporary build tags to ensure clean teardown.

### 2.4 Orchestrator (`internal/orchestrator`)
- Coordinates the complete lifecycle:
  1. Docker daemon preflight check (bypassed on `--dry-run` or `--no-build`).
  2. Project scan.
  3. LLM / heuristic generation.
  4. Plan presentation & interactive confirmation.
  5. Disk writes (with `.bak` backup preservation).
  6. Multi-attempt build verification & retry loop.
  7. Container startup & healthcheck verification.
  8. Linter integration (`hadolint`, `actionlint`).

## 3. Security Boundary
- **Sandboxed File Operations:** Prevents directory traversal (`../`) outside project root.
- **Secret Leak Prevention:** Enforces pre-read and pre-upload stripping of sensitive certificates, SSH keys, and password values.
