# Dockship Containerization: sample-node-app

This project was containerized by **Dockship**.

## Detected Stack
- **Language / Runtime:** Node.js (20-alpine)
- **Framework:** Express
- **Package Manager:** npm
- **Exposed Port(s):** 3000

## Generated Artifacts
- `Dockerfile`: Multi-stage, non-root, cache-optimized image
- `.dockerignore`: Prevents bloat and keeps credentials safe
- `docker-compose.yml`: Local multi-service environment
- `.github/workflows/ci.yml`: GitHub Actions build & push pipeline

## How to Run Locally

### Using Docker
```bash
docker build -t sample-node-app .
docker run -p 3000:3000 sample-node-app
```

### Using Docker Compose
```bash
docker compose up --build -d
```

## Required CI Secrets
- Automatically uses default `GITHUB_TOKEN` for `ghcr.io`
