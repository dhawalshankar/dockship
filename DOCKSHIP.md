# Dockship Containerization: dockship

This project was containerized by **Dockship**.

## Detected Stack
- **Language / Runtime:** Go (1.26.5-alpine)
- **Framework:** Cobra CLI
- **Package Manager:** go mod
- **Application Type:** CLI / Worker (no network ports exposed)

## Generated Artifacts
- `Dockerfile`: Multi-stage, non-root, cache-optimized image
- `.dockerignore`: Prevents bloat and keeps credentials safe
- `.github/workflows/ci.yml`: GitHub Actions build & push pipeline

## How to Run Locally

### Using Docker
```bash
docker build -t dockship .
docker run --rm dockship --help
```

## Required CI Secrets
- Automatically uses default `GITHUB_TOKEN` for `ghcr.io`
