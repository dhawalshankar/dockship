# Dockship CLI Reference Manual

## Synopsis
```bash
dockship [path-to-project] [flags]
```
If `path-to-project` is omitted, Dockship targets the current working directory (`.`).

---

## Flags

### CI/CD & Registry
- `--ci <provider>`: CI/CD workflow provider.
  - Options: `github` (default), `gitlab`, `none`.
- `--registry <name>`: Target container registry.
  - Options: `ghcr` (default), `dockerhub`, `ecr`, `gcr`, `none`.
- `--compose <auto|true|false>`: Controls `docker-compose.yml` generation.
  - `auto` (default): Generated if multiple services or external databases/caches are detected.
  - `true`: Forces compose generation.
  - `false`: Disables compose generation.

### Execution & Safety
- `-y`, `--yes`: Skip interactive confirmation prompt and write all generated files immediately.
- `--dry-run`: Output generated file contents to the terminal without modifying any disk files. Skips Docker daemon requirement.
- `--no-build`: Skip Docker build and container healthcheck validation steps.
- `--force`: Overwrite existing Docker and CI/CD files without prompting.

### LLM & Diagnostics
- `--model <id>`: Override the default LLM model (default: `openai/gpt-oss-120b`).
- `--max-iterations <n>`: Maximum number of agent repair iterations (default: `8`).
- `--verbose`: Stream live agent reasoning, tool invocations, and tool results.
- `--json`: Format all final outputs, errors, and metadata as structured JSON for automation.
- `-v`, `--version`: Print Dockship version.
- `-h`, `--help`: Help documentation.

---

## Environment Variables
- `GROQ_API_KEY`: Groq API token. (Optional: Dockship operates with 100% functionality offline without this key).
- `OPENAI_API_KEY`: Fallback OpenAI token if Groq is not used.
- `GROQ_BASE_URL`: Custom base URL for OpenAI-compatible endpoints.
- `DOCKSHIP_MODEL`: Override default model globally.

---

## Project Configuration (`.dockship.yml`)
Place `.dockship.yml` in the project root to enforce persistent team settings:
```yaml
ci: github
registry: ghcr
compose: auto
model: openai/gpt-oss-120b
overrides:
  ports: [8080]
  base_image: alpine:3.19
```

---

## Exit Codes
| Code | Meaning |
|---|---|
| `0` | Success: Image built and validated (or dry-run finished). |
| `1` | Error: Docker daemon unreachable, build failure, or validation timeout. |

---

## Troubleshooting Guide

### 1. Docker daemon connection errors
**Error:** `failed to connect to the docker API at npipe:////./pipe/docker_engine`  
**Fix:** Ensure Docker Desktop or the Docker daemon is started. If you only want to generate files without running Docker, pass `--no-build` or `--dry-run`.

### 2. Groq 429 Rate Limits
**Message:** `Rate limit reached for model openai/gpt-oss-120b ... on TPM`  
**Fix:** Dockship automatically handles this by seamlessly switching to its offline heuristics engine, so your workflow is never blocked.

### 3. Chrome Extensions & Static Sites
Dockship auto-detects `manifest.json` and static HTML/JS projects, containerizing them in an ultra-fast `nginx:alpine` image on port 80.
