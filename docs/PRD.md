# Dockship — Product Requirements Document

**Version:** 0.2  
**Tech Stack:** Go (1.22+)  
**Scope:** Build, Validate, and Registry Push (Deployment out of scope)

---

## 1. Overview
**Dockship** is an AI-powered CLI that takes any project (any language, any framework) and fully containerizes it, including CI/CD setup, with a single command:
```bash
dockship <path-to-project>
```
An LLM agent (Groq-hosted `gpt-oss-120b` or built-in heuristics engine) analyzes the codebase, generates Docker and CI/CD artifacts, and **validates them by building and running the container** before declaring success.

## 2. Problem Statement
Dockerizing a project is repetitive and error-prone:
- Each stack needs different base images, build steps, ports, and env handling.
- Developers write insecure or bloated Dockerfiles (root user, no multi-stage, no `.dockerignore`, huge images).
- CI/CD pipelines (build, test, push) are written from scratch per project.
- Templates break on non-standard project layouts (monorepos, custom start commands, CLI tools).

## 3. Goals and Non-Goals
### Goals
1. One command takes a project from "no Docker" to "builds, runs, and has CI/CD".
2. Works across languages and frameworks without per-stack hard-coding. The LLM does the reasoning.
3. Output is verified: image builds and container starts before success is reported.
4. Generated artifacts follow best practices (multi-stage, non-root, layer caching, small images, healthchecks).
5. Transparent: show a plan and diff, let the user review before writing (configurable).

### Non-Goals (v1)
- Kubernetes manifests / Helm charts.
- Provisioning cloud infrastructure (Terraform etc.).
- **Deployment**: Dockship stops at build, validate, and registry push. No deploy steps or deploy stubs.
- Refactoring application code to make it "container-friendly".
- Acting as a general-purpose coding agent.

## 4. Target Users
- Solo dev / student: Wants Docker without learning every detail.
- Startup team: Fast CI/CD setup for many small services.
- DevOps engineer: Bootstrap baseline Dockerfiles across many repos, then tweak.

## 5. Functional Requirements
- **FR1. Project Analysis:** Walk directory, respect `.gitignore`, detect language, framework, package manager, runtime version, build & start commands, ports, env vars, external services.
- **FR2. Artifact Generation:** `Dockerfile`, `.dockerignore`, `docker-compose.yml`, CI/CD workflow, `.env.example`, `DOCKSHIP.md`.
- **FR3. Best Practices Enforcement:** Pinned base images, non-root user, layer caching, healthchecks, no secrets in image.
- **FR4. Validation Loop:** Run `docker build`, patch on failure, run container, check health/port, cleanup test containers.
- **FR5. CI/CD Generation:** GitHub Actions, GitLab CI. Push to registry (GHCR, DockerHub, ECR, GCR).
- **FR6. Safety and Control:** Target project sandboxed writes, allowlisted tools, never read or leak raw secrets.
- **FR7. Configuration:** `GROQ_API_KEY`, optional `.dockship.yml`.
- **FR8. Output and Observability:** Step-by-step progress, `--verbose`, `--json`.
