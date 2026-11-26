package agent

import (
	"fmt"
	"strings"

	"github.com/dockship/dockship/internal/scanner"
)

// GeneratedArtifacts holds the file paths and contents.
type GeneratedArtifacts struct {
	Files map[string]string // relPath -> content
}

// GenerateHeuristicArtifacts provides a production-grade baseline generator.
func GenerateHeuristicArtifacts(info *scanner.ProjectInfo, ciProvider, registry string, forceCompose bool) *GeneratedArtifacts {
	artifacts := &GeneratedArtifacts{
		Files: make(map[string]string),
	}

	primarySvc := scanner.ServiceInfo{
		Name:           info.ProjectName,
		Language:       "Node.js",
		RuntimeVersion: "20-alpine",
		Ports:          []int{3000},
	}
	if len(info.Services) > 0 {
		primarySvc = info.Services[0]
	}

	// 1. Generate Dockerfile
	artifacts.Files["Dockerfile"] = generateDockerfile(&primarySvc)

	// 2. Generate .dockerignore
	artifacts.Files[".dockerignore"] = generateDockerignore()

	// 3. Generate docker-compose.yml if services or dependencies exist or forced
	needCompose := forceCompose || len(info.ExternalServices) > 0 || len(info.Services) > 1
	if needCompose {
		artifacts.Files["docker-compose.yml"] = generateDockerCompose(info, &primarySvc)
	}

	// 4. Generate CI/CD workflow
	switch strings.ToLower(ciProvider) {
	case "github":
		artifacts.Files[".github/workflows/ci.yml"] = generateGitHubActionsCI(info, registry)
	case "gitlab":
		artifacts.Files[".gitlab-ci.yml"] = generateGitLabCI(info, registry)
	}

	// 5. Generate .env.example if env vars detected
	if len(info.EnvVars) > 0 {
		artifacts.Files[".env.example"] = generateEnvExample(info)
	}

	// 6. Generate DOCKSHIP.md
	artifacts.Files["DOCKSHIP.md"] = generateDockshipDoc(info, &primarySvc, ciProvider, registry, needCompose)

	return artifacts
}

func generateDockerfile(svc *scanner.ServiceInfo) string {
	switch strings.ToLower(svc.Language) {
	case "node.js", "javascript", "typescript":
		return generateNodeDockerfile(svc)
	case "python":
		return generatePythonDockerfile(svc)
	case "go", "golang":
		return generateGoDockerfile(svc)
	case "rust":
		return generateRustDockerfile(svc)
	case "java":
		return generateJavaDockerfile(svc)
	case "php":
		return generatePHPDockerfile(svc)
	case "c# / .net", ".net":
		return generateDotnetDockerfile(svc)
	default:
		return generateGenericDockerfile(svc)
	}
}

func generateNodeDockerfile(svc *scanner.ServiceInfo) string {
	nodeVer := svc.RuntimeVersion
	if nodeVer == "" || !strings.Contains(nodeVer, "alpine") {
		nodeVer = "20-alpine"
	}
	port := 3000
	if len(svc.Ports) > 0 {
		port = svc.Ports[0]
	}

	var installCmd, lockfile string
	switch svc.PackageManager {
	case "pnpm":
		installCmd = "RUN corepack enable && corepack prepare pnpm@latest --activate && pnpm install --frozen-lockfile"
		lockfile = "pnpm-lock.yaml"
	case "yarn":
		installCmd = "RUN yarn install --frozen-lockfile"
		lockfile = "yarn.lock"
	case "bun":
		installCmd = "RUN bun install --frozen-lockfile"
		lockfile = "bun.lockb"
	default:
		installCmd = "RUN npm ci --omit=dev || npm install"
		lockfile = "package-lock.json"
	}

	startCmd := svc.StartCommand
	if startCmd == "" {
		startCmd = "npm start"
	}
	cmdParts := strings.Fields(startCmd)
	cmdJSON := formatJSONArgs(cmdParts)

	return fmt.Sprintf(`# Multi-stage production Dockerfile for Node.js
# Stage 1: Dependencies & Build
FROM node:%s AS builder
WORKDIR /app

# Layer caching for dependency definitions
COPY package.json %s* ./
%s

# Copy source code and build
COPY . .
%s

# Stage 2: Minimal Production Runtime
FROM node:%s AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=%d

# Use non-root user
USER node

COPY --from=builder --chown=node:node /app ./

EXPOSE %d

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:%d/ || exit 1

CMD %s
`, nodeVer, lockfile, installCmd, getBuildStep(svc), nodeVer, port, port, port, cmdJSON)
}

func getBuildStep(svc *scanner.ServiceInfo) string {
	if svc.BuildCommand != "" {
		return fmt.Sprintf("RUN %s", svc.BuildCommand)
	}
	return "# No build step specified"
}

func generatePythonDockerfile(svc *scanner.ServiceInfo) string {
	pyVer := svc.RuntimeVersion
	if pyVer == "" {
		pyVer = "3.11-slim"
	}
	port := 8000
	if len(svc.Ports) > 0 {
		port = svc.Ports[0]
	}

	startCmd := svc.StartCommand
	if startCmd == "" {
		startCmd = "python app.py"
	}
	cmdParts := strings.Fields(startCmd)
	cmdJSON := formatJSONArgs(cmdParts)

	return fmt.Sprintf(`# Multi-stage production Dockerfile for Python
# Stage 1: Build virtual environment
FROM python:%s AS builder
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends gcc build-essential && rm -rf /var/lib/apt/lists/*

RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

COPY requirements.txt* pyproject.toml* ./
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi

# Stage 2: Minimal Production Image
FROM python:%s AS runner
WORKDIR /app

# Security: Create and switch to non-root user
RUN groupadd -r appuser && useradd -r -g appuser appuser

ENV PATH="/opt/venv/bin:$PATH"
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV PORT=%d

COPY --from=builder /opt/venv /opt/venv
COPY --chown=appuser:appuser . .

USER appuser

EXPOSE %d

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:%d/').read()" || exit 1

CMD %s
`, pyVer, pyVer, port, port, port, cmdJSON)
}

func generateGoDockerfile(svc *scanner.ServiceInfo) string {
	goVer := svc.RuntimeVersion
	if goVer == "" {
		goVer = "1.22-alpine"
	}

	binaryPath := svc.StartCommand
	if binaryPath == "" {
		binaryPath = "/app/app"
	}
	buildCmd := svc.BuildCommand
	if buildCmd == "" {
		buildCmd = "CGO_ENABLED=0 GOOS=linux go build -ldflags=\"-w -s\" -o /app/app ."
	}

	var portBlock, healthBlock, cmdBlock string
	if len(svc.Ports) > 0 {
		port := svc.Ports[0]
		portBlock = fmt.Sprintf("ENV PORT=%d\nEXPOSE %d\n", port, port)
		healthBlock = fmt.Sprintf("HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\\n  CMD wget --no-verbose --tries=1 --spider http://localhost:%d/ || exit 1\n", port)
	} else {
		cmdBlock = "CMD [\"--help\"]\n"
	}

	return fmt.Sprintf(`# Multi-stage production Dockerfile for Go
# Stage 1: Build binary
FROM golang:%s AS builder
WORKDIR /app

# Download dependencies first for caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source and compile statically
COPY . .
RUN %s

# Stage 2: Minimal secure runtime (scratch or alpine)
FROM alpine:3.19 AS runner
WORKDIR /app

# Security: unprivileged user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

COPY --from=builder %s %s

%s%s
ENTRYPOINT ["%s"]
%s`, goVer, buildCmd, binaryPath, binaryPath, portBlock, healthBlock, binaryPath, cmdBlock)
}

func generateRustDockerfile(svc *scanner.ServiceInfo) string {
	port := 8080
	if len(svc.Ports) > 0 {
		port = svc.Ports[0]
	}
	return fmt.Sprintf(`# Multi-stage production Dockerfile for Rust
FROM rust:alpine AS builder
WORKDIR /app
RUN apk add --no-cache musl-dev
COPY Cargo.toml Cargo.lock* ./
COPY src ./src
RUN cargo build --release

FROM alpine:3.19 AS runner
WORKDIR /app
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
COPY --from=builder /app/target/release/* /app/app
EXPOSE %d
ENTRYPOINT ["/app/app"]
`, port)
}

func generateJavaDockerfile(svc *scanner.ServiceInfo) string {
	port := 8080
	if len(svc.Ports) > 0 {
		port = svc.Ports[0]
	}
	return fmt.Sprintf(`# Multi-stage production Dockerfile for Java / Spring Boot
FROM eclipse-temurin:17-jdk-alpine AS builder
WORKDIR /app
COPY pom.xml* mvnw* gradlew* ./
COPY .mvn .mvn/
COPY src ./src
RUN if [ -f mvnw ]; then ./mvnw clean package -DskipTests; elif [ -f gradlew ]; then ./gradlew build -x test; fi

FROM eclipse-temurin:17-jre-alpine AS runner
WORKDIR /app
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
COPY --from=builder /app/target/*.jar /app/app.jar || COPY --from=builder /app/build/libs/*.jar /app/app.jar
EXPOSE %d
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
`, port)
}

func generatePHPDockerfile(svc *scanner.ServiceInfo) string {
	port := 8000
	if len(svc.Ports) > 0 {
		port = svc.Ports[0]
	}
	return fmt.Sprintf(`# Multi-stage production Dockerfile for PHP
FROM php:8.2-fpm-alpine AS runner
WORKDIR /var/www/html
RUN apk add --no-cache curl
COPY . .
EXPOSE %d
CMD ["php", "-S", "0.0.0.0:%d", "-t", "public"]
`, port, port)
}

func generateDotnetDockerfile(svc *scanner.ServiceInfo) string {
	port := 8080
	if len(svc.Ports) > 0 {
		port = svc.Ports[0]
	}
	return fmt.Sprintf(`# Multi-stage production Dockerfile for .NET Core
FROM mcr.microsoft.com/dotnet/sdk:8.0 AS builder
WORKDIR /source
COPY *.csproj ./
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o /app

FROM mcr.microsoft.com/dotnet/aspnet:8.0 AS runner
WORKDIR /app
COPY --from=builder /app ./
EXPOSE %d
ENTRYPOINT ["dotnet", "app.dll"]
`, port)
}

func generateGenericDockerfile(svc *scanner.ServiceInfo) string {
	port := 8080
	if len(svc.Ports) > 0 {
		port = svc.Ports[0]
	}
	return fmt.Sprintf(`# Generic container configuration
FROM alpine:3.19
WORKDIR /app
RUN apk add --no-cache bash ca-certificates
COPY . .
EXPOSE %d
CMD ["sh", "-c", "echo Dockship generic container running; sleep infinity"]
`, port)
}

func generateDockerignore() string {
	return `# Git
.git
.gitignore

# CI/CD and Dockship
.github
.gitlab
DOCKSHIP.md
.gotmp/
.dockship_tmp/
bin/
*.exe
*.test

# Dependencies & Build Artefacts
node_modules/
vendor/
target/
obj/
dist/
build/
.next/
.nuxt/
.cache/
__pycache__/
*.pyc
.venv/
venv/

# Environment and sensitive secrets
.env
.env.*
!.env.example
*.pem
*.key
*.pfx
*.p12
id_rsa
credentials

# OS and Editor metadata
.DS_Store
Thumbs.db
.idea/
.vscode/
*.swp
*.log
`
}

func generateDockerCompose(info *scanner.ProjectInfo, primarySvc *scanner.ServiceInfo) string {
	port := 3000
	if len(primarySvc.Ports) > 0 {
		port = primarySvc.Ports[0]
	}

	var sb strings.Builder
	sb.WriteString("version: '3.8'\n\nservices:\n")
	sb.WriteString(fmt.Sprintf("  app:\n    build:\n      context: .\n      dockerfile: Dockerfile\n    ports:\n      - \"%d:%d\"\n    restart: unless-stopped\n", port, port))

	if len(info.ExternalServices) > 0 {
		sb.WriteString("    depends_on:\n")
		for _, ext := range info.ExternalServices {
			sb.WriteString(fmt.Sprintf("      - %s\n", ext))
		}
	}

	sb.WriteString("    environment:\n")
	if len(info.EnvVars) > 0 {
		for _, e := range info.EnvVars {
			sb.WriteString(fmt.Sprintf("      - %s=${%s}\n", e, e))
		}
	} else {
		sb.WriteString("      - NODE_ENV=production\n")
	}

	// External databases / caches
	for _, ext := range info.ExternalServices {
		switch ext {
		case "postgres":
			sb.WriteString(`
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-postgres}
      POSTGRES_DB: ${POSTGRES_DB:-app}
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
`)
		case "redis":
			sb.WriteString(`
  redis:
    image: redis:7-alpine
    restart: unless-stopped
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
`)
		case "mongodb":
			sb.WriteString(`
  mongodb:
    image: mongo:7-jammy
    restart: unless-stopped
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db
`)
		case "mysql":
			sb.WriteString(`
  mysql:
    image: mysql:8.0
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD:-password}
      MYSQL_DATABASE: ${MYSQL_DATABASE:-app}
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
`)
		case "rabbitmq":
			sb.WriteString(`
  rabbitmq:
    image: rabbitmq:3-management-alpine
    restart: unless-stopped
    ports:
      - "5672:5672"
      - "15672:15672"
`)
		}
	}

	// Volumes
	if len(info.ExternalServices) > 0 {
		sb.WriteString("\nvolumes:\n")
		for _, ext := range info.ExternalServices {
			if ext == "postgres" {
				sb.WriteString("  postgres_data:\n")
			} else if ext == "redis" {
				sb.WriteString("  redis_data:\n")
			} else if ext == "mongodb" {
				sb.WriteString("  mongo_data:\n")
			} else if ext == "mysql" {
				sb.WriteString("  mysql_data:\n")
			}
		}
	}

	return sb.String()
}

func generateGitHubActionsCI(info *scanner.ProjectInfo, registry string) string {
	reg := strings.ToLower(registry)
	regServer := "ghcr.io"
	userVar := "${{ github.actor }}"
	passVar := "${{ secrets.GITHUB_TOKEN }}"
	imagePrefix := "ghcr.io/${{ github.repository }}"

	if reg == "dockerhub" {
		regServer = "docker.io"
		userVar = "${{ secrets.DOCKERHUB_USERNAME }}"
		passVar = "${{ secrets.DOCKERHUB_TOKEN }}"
		imagePrefix = "${{ secrets.DOCKERHUB_USERNAME }}/" + info.ProjectName
	}

	return fmt.Sprintf(`name: ci
on:
  push:
    branches: [main, master]
    tags: ['v*.*.*']
  pull_request:
    branches: [main, master]

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Registry
        if: github.event_name != 'pull_request'
        uses: docker/login-action@v3
        with:
          registry: %s
          username: %s
          password: %s

      - name: Extract Docker metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: %s
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=semver,pattern={{version}}
            type=sha,format=short

      - name: Build and push image
        uses: docker/build-push-action@v6
        with:
          context: .
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
`, regServer, userVar, passVar, imagePrefix)
}

func generateGitLabCI(info *scanner.ProjectInfo, registry string) string {
	return `stages:
  - build

variables:
  DOCKER_DRIVER: overlay2
  DOCKER_TLS_CERTDIR: ""

build-image:
  stage: build
  image: docker:24.0.5
  services:
    - docker:24.0.5-dind
  before_script:
    - echo "$CI_REGISTRY_PASSWORD" | docker login -u "$CI_REGISTRY_USER" --password-stdin "$CI_REGISTRY"
  script:
    - docker build -t "$CI_REGISTRY_IMAGE:$CI_COMMIT_SHA" -t "$CI_REGISTRY_IMAGE:latest" .
    - docker push "$CI_REGISTRY_IMAGE:$CI_COMMIT_SHA"
    - docker push "$CI_REGISTRY_IMAGE:latest"
  only:
    - main
    - master
    - tags
`
}

func generateEnvExample(info *scanner.ProjectInfo) string {
	var sb strings.Builder
	sb.WriteString("# Environment configuration template generated by Dockship\n")
	sb.WriteString("# Copy to .env and configure secrets accordingly\n\n")
	for _, env := range info.EnvVars {
		placeholder := "your_" + strings.ToLower(env)
		if strings.Contains(env, "PORT") {
			placeholder = "3000"
		} else if strings.Contains(env, "DATABASE_URL") {
			placeholder = "postgres://postgres:postgres@localhost:5432/app"
		} else if strings.Contains(env, "REDIS_URL") {
			placeholder = "redis://localhost:6379"
		}
		sb.WriteString(fmt.Sprintf("%s=%s\n", env, placeholder))
	}
	return sb.String()
}

func generateDockshipDoc(info *scanner.ProjectInfo, svc *scanner.ServiceInfo, ciProvider, registry string, hasCompose bool) string {
	port := 3000
	if len(svc.Ports) > 0 {
		port = svc.Ports[0]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Dockship Containerization: %s\n\n", info.ProjectName))
	sb.WriteString("This project was containerized by **Dockship**.\n\n")
	sb.WriteString("## Detected Stack\n")
	sb.WriteString(fmt.Sprintf("- **Language / Runtime:** %s (%s)\n", svc.Language, svc.RuntimeVersion))
	if svc.Framework != "" {
		sb.WriteString(fmt.Sprintf("- **Framework:** %s\n", svc.Framework))
	}
	if svc.PackageManager != "" {
		sb.WriteString(fmt.Sprintf("- **Package Manager:** %s\n", svc.PackageManager))
	}
	sb.WriteString(fmt.Sprintf("- **Exposed Port(s):** %d\n\n", port))

	sb.WriteString("## Generated Artifacts\n")
	sb.WriteString("- `Dockerfile`: Multi-stage, non-root, cache-optimized image\n")
	sb.WriteString("- `.dockerignore`: Prevents bloat and keeps credentials safe\n")
	if hasCompose {
		sb.WriteString("- `docker-compose.yml`: Local multi-service environment\n")
	}
	if ciProvider == "github" {
		sb.WriteString("- `.github/workflows/ci.yml`: GitHub Actions build & push pipeline\n")
	} else if ciProvider == "gitlab" {
		sb.WriteString("- `.gitlab-ci.yml`: GitLab CI build & push pipeline\n")
	}
	sb.WriteString("\n## How to Run Locally\n\n")
	sb.WriteString("### Using Docker\n```bash\n")
	sb.WriteString(fmt.Sprintf("docker build -t %s .\n", strings.ToLower(info.ProjectName)))
	sb.WriteString(fmt.Sprintf("docker run -p %d:%d %s\n", port, port, strings.ToLower(info.ProjectName)))
	sb.WriteString("```\n\n")

	if hasCompose {
		sb.WriteString("### Using Docker Compose\n```bash\ndocker compose up --build -d\n```\n\n")
	}

	if ciProvider == "github" {
		sb.WriteString("## Required CI Secrets\n")
		if strings.ToLower(registry) == "dockerhub" {
			sb.WriteString("- `DOCKERHUB_USERNAME`: Your DockerHub username\n")
			sb.WriteString("- `DOCKERHUB_TOKEN`: DockerHub Personal Access Token\n")
		} else {
			sb.WriteString("- Automatically uses default `GITHUB_TOKEN` for `ghcr.io`\n")
		}
	}

	return sb.String()
}

func formatJSONArgs(parts []string) string {
	escaped := make([]string, len(parts))
	for i, p := range parts {
		escaped[i] = fmt.Sprintf(`"%s"`, strings.ReplaceAll(p, `"`, `\"`))
	}
	return "[" + strings.Join(escaped, ", ") + "]"
}
