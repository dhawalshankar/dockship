# Dockship 🚀

**Dockship** is an AI-powered CLI that takes any project (any language, any framework) and fully containerizes it, including CI/CD setup, with a single command:

```bash
dockship <path-to-project>
```

An LLM agent (Groq-hosted `gpt-oss-120b` or built-in heuristics engine) analyzes the codebase, generates production-grade Docker and CI/CD artifacts, and **validates them by building and running the container** before declaring success.

---

## ✨ Features

- **Zero-Config Multi-Stack Analysis:** Detects Node.js, Python, Go, Java, Rust, PHP, .NET, Ruby, package managers, runtime versions, entrypoints, and ports.
- **Production-Grade Dockerfiles:** Multi-stage builds, non-root unprivileged users, optimal layer caching, and healthchecks.
- **Security & Privacy First:** Sensitive secrets (`.env` values, `.pem`, `.key`, `id_rsa`) are never uploaded or read.
- **Full CI/CD Pipeline:** Ready-to-go GitHub Actions (`.github/workflows/ci.yml`) or GitLab CI (`.gitlab-ci.yml`) targeting GHCR, DockerHub, or custom registries.
- **Microservices & Databases:** Auto-detects dependencies (`DATABASE_URL`, `REDIS_URL`, Mongo, MySQL, RabbitMQ) and generates `docker-compose.yml`.
- **Validation Loop:** Actually tests `docker build` and `docker run` locally, capturing logs and auto-patching errors before reporting success.
- **Transparent & Safe:** Interactive plan confirmation, `--dry-run` previews, backups (`*.bak`), and optional `--json` output for automation.

---

## 📦 Installation & Build

Build the static binary:

```bash
go build -o dockship.exe ./cmd/dockship
```

Or install globally:

```bash
go install ./cmd/dockship
```

---

## 🚀 Usage

### Basic Usage
```bash
dockship ./my-project
```

### Flags & Options
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

## ⚙ Configuration

### Environment Variables
- `GROQ_API_KEY`: API key for Groq LLM (default model `openai/gpt-oss-120b`).
  - *Note:* If no key is set, Dockship seamlessly switches to its built-in expert heuristics engine.
- `DOCKSHIP_MODEL`: Override the default LLM model.
- `GROQ_BASE_URL`: Override API base URL.

### Project Overrides (`.dockship.yml`)
You can add a `.dockship.yml` file to the root of any project to customize defaults:

```yaml
ci: github
registry: ghcr
compose: auto
overrides:
  ports: [3000]
  base_image: node:20-alpine
```

---

## 🛠 Architecture

```
CLI (Cobra UX)
  └─ Orchestrator
       ├─ Scanner         (file tree, manifest parsing, secrets filter)
       ├─ Agent loop      (Groq gpt-oss / OpenAI API, tool calling)
       │    └─ Tools      (list_dir, read_file, search, write_file, docker_build, docker_run)
       ├─ Validator       (docker build, container healthcheck, hadolint/actionlint)
       └─ Reporter        (terminal colors, diff preview, JSON output)
```

---

## 📄 License

Apache-2.0 / MIT
