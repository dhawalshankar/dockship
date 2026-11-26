# Dockship 🚀

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![CI/CD](https://img.shields.io/badge/CI%2FCD-GitHub%20%7C%20GitLab-2088FF?style=flat)](https://github.com/features/actions)

**Dockship** is an AI-powered CLI that takes any project (any language, any framework) and fully containerizes it, including CI/CD setup, with a single command:

```bash
dockship <path-to-project>
```

An LLM agent (Groq-hosted `gpt-oss-120b` or built-in heuristics engine) analyzes the codebase, generates production-grade Docker and CI/CD artifacts, and **validates them by actually building and running the container** before declaring success.

---

## ✨ Features

- **Multi-Stack Auto Detection:** Detects Node.js, Python, Go, Java, Rust, PHP, .NET, Ruby, Chrome Extensions, Static Web apps, package managers, runtime versions, entrypoints, and ports.
- **Production-Grade Dockerfiles:** Multi-stage builds, non-root unprivileged users, optimal layer caching, and healthchecks.
- **Fail-Safe & Rate-Limit Resilient:** Seamlessly falls back to an expert offline heuristics engine if Groq/LLM encounters 429 rate limits or network issues. No API key required to get production outputs!
- **Chrome Extensions & Static Web Support:** Auto-generates super-fast, ultra-lightweight `nginx:alpine` containers for browser extensions and static frontends with built-in healthchecks.
- **Security & Privacy First:** Sensitive secrets (`.env` values, `.pem`, `.key`, `id_rsa`) are never uploaded or read.
- **Full CI/CD Pipeline:** Ready-to-go GitHub Actions (`.github/workflows/ci.yml`) or GitLab CI (`.gitlab-ci.yml`) targeting GHCR, DockerHub, or custom registries.
- **Microservices & Databases:** Auto-detects dependencies (`DATABASE_URL`, `REDIS_URL`, Mongo, MySQL, RabbitMQ) and generates `docker-compose.yml`.
- **Closed Validation Loop:** Actually tests `docker build` and `docker run` locally, capturing logs and auto-patching errors before reporting success.
- **Transparent & Safe:** Interactive plan confirmation, `--dry-run` previews, backups (`*.bak`), and optional `--json` output for automation.

---

## 📦 Installation

### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/dockship/dockship/main/install.ps1 | iex
```
*Or build locally:*
```powershell
go build -o bin/dockship.exe ./cmd/dockship
Set-Alias dockship (Resolve-Path .\bin\dockship.exe)
```

### Linux / macOS
```bash
curl -fsSL https://raw.githubusercontent.com/dockship/dockship/main/install.sh | bash
```

### Via `go install`
```bash
go install github.com/dockship/dockship/cmd/dockship@latest
```

---

## 🚀 Usage

### 1. Basic Run
Containerize any local repository or directory:
```bash
dockship ./my-project
```

### 2. Preview (Dry-Run)
Inspect what Dockerfile and CI files will look like without writing to disk:
```bash
dockship ./my-project --dry-run
```

### 3. Non-Interactive / Automation
Skip confirmation prompt and overwrite existing setups:
```bash
dockship ./my-project --yes --force
```

---

## ⚙ Flags & Options

| Flag | Description | Default |
|---|---|---|
| `--ci <provider>` | `github`, `gitlab`, `none` | `github` |
| `--registry <name>` | `ghcr`, `dockerhub`, `ecr`, `gcr`, `none` | `ghcr` |
| `--yes`, `-y` | Skip confirmation prompt, write all files | `off` |
| `--dry-run` | Preview generated files without writing | `off` |
| `--no-build` | Skip docker build and run validation | `off` |
| `--compose` | Force docker-compose generation (`auto`, `true`, `false`) | `auto` |
| `--model <id>` | Override LLM model | `openai/gpt-oss-120b` |
| `--max-iterations <n>` | Max agent fix-loop iterations | `8` |
| `--verbose` | Show agent thoughts and tool calls | `off` |
| `--force` | Overwrite existing Docker/CI files | `off` |
| `--json` | Output machine-readable JSON | `off` |

---

## 💡 Real-World Case Studies

### Case 1: Chrome Extension (`kimbho-translator`)
- **Challenge:** Browser extensions have `manifest.json` and static HTML/JS/CSS assets without a standard Node.js server or `package.json`.
- **Dockship Solution:** Auto-detected `Manifest V3`, generated an ultra-lightweight `nginx:alpine` container (`27.7 MB`), exposed port 80, and verified with container healthchecks in 24 seconds!

### Case 2: Dockship (Go CLI Dogfooding)
- **Challenge:** Go CLI with entrypoint at `./cmd/dockship/main.go` and no listening network ports.
- **Dockship Solution:** Inspected `cmd/*` to locate entrypoint, generated static CGO-free multi-stage alpine container with `ENTRYPOINT ["/app/dockship"]` and `CMD ["--help"]` without fake web healthchecks.

---

## 📄 Documentation

- [Product Requirements Document (PRD)](docs/PRD.md)
- [Architecture & Technical Design](docs/ARCHITECTURE.md)
- [CLI Reference Manual](docs/CLI_REFERENCE.md)
- [Contributing Guidelines](CONTRIBUTING.md)

---

## 🛡 License

Licensed under [Apache-2.0](LICENSE).
