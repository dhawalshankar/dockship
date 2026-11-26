package scanner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var portRegex = regexp.MustCompile(`(?i)(?:port|listen|addr|listenandserve|bind)[\s:=("']+(\d{2,5})`)
var secretPatternRegex = regexp.MustCompile(`(?i)(password|secret|key|token|auth|credential|cert|private)`)

// DetectService scans a specific directory (project root or service subdirectory)
// and returns ServiceInfo if recognizable code/manifests exist.
func DetectService(dirPath, relPath string) *ServiceInfo {
	svc := &ServiceInfo{
		Name:     filepath.Base(dirPath),
		Path:     relPath,
		Ports:    make([]int, 0),
		EnvVars:  make([]string, 0),
	}

	found := false

	// 1. Node.js / TypeScript
	pkgJsonPath := filepath.Join(dirPath, "package.json")
	if info, err := os.Stat(pkgJsonPath); err == nil && !info.IsDir() {
		found = true
		svc.Language = "Node.js"
		parseNodeManifest(dirPath, pkgJsonPath, svc)
	}

	// 2. Python
	pyProject := filepath.Join(dirPath, "pyproject.toml")
	reqTxt := filepath.Join(dirPath, "requirements.txt")
	pipfile := filepath.Join(dirPath, "Pipfile")
	setupPy := filepath.Join(dirPath, "setup.py")
	if fileExists(pyProject) || fileExists(reqTxt) || fileExists(pipfile) || fileExists(setupPy) {
		found = true
		if svc.Language == "" {
			svc.Language = "Python"
		}
		parsePythonManifest(dirPath, svc)
	}

	// 3. Go
	goModPath := filepath.Join(dirPath, "go.mod")
	if fileExists(goModPath) {
		found = true
		if svc.Language == "" {
			svc.Language = "Go"
		}
		parseGoManifest(goModPath, svc)
	}

	// 4. Rust
	cargoPath := filepath.Join(dirPath, "Cargo.toml")
	if fileExists(cargoPath) {
		found = true
		if svc.Language == "" {
			svc.Language = "Rust"
		}
		parseRustManifest(cargoPath, svc)
	}

	// 5. Java
	pomPath := filepath.Join(dirPath, "pom.xml")
	gradlePath := filepath.Join(dirPath, "build.gradle")
	gradleKtsPath := filepath.Join(dirPath, "build.gradle.kts")
	if fileExists(pomPath) || fileExists(gradlePath) || fileExists(gradleKtsPath) {
		found = true
		if svc.Language == "" {
			svc.Language = "Java"
		}
		parseJavaManifest(dirPath, pomPath, gradlePath, gradleKtsPath, svc)
	}

	// 6. PHP
	composerPath := filepath.Join(dirPath, "composer.json")
	if fileExists(composerPath) {
		found = true
		if svc.Language == "" {
			svc.Language = "PHP"
		}
		parsePHPManifest(composerPath, svc)
	}

	// 7. Ruby
	gemfilePath := filepath.Join(dirPath, "Gemfile")
	if fileExists(gemfilePath) {
		found = true
		if svc.Language == "" {
			svc.Language = "Ruby"
		}
		parseRubyManifest(gemfilePath, svc)
	}

	// 8. .NET
	if matches, _ := filepath.Glob(filepath.Join(dirPath, "*.csproj")); len(matches) > 0 {
		found = true
		if svc.Language == "" {
			svc.Language = "C# / .NET"
		}
		svc.Framework = ".NET Core"
		svc.BuildCommand = "dotnet publish -c Release -o out"
		svc.StartCommand = "dotnet out/" + strings.TrimSuffix(filepath.Base(matches[0]), ".csproj") + ".dll"
		addPortIfMissing(svc, 8080)
	}

	if !found {
		return nil
	}

	// Scan source files lightly for port mentions if none found
	if len(svc.Ports) == 0 {
		detectPortsFromSource(dirPath, svc)
	}

	return svc
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func addPortIfMissing(svc *ServiceInfo, port int) {
	for _, p := range svc.Ports {
		if p == port {
			return
		}
	}
	if port > 0 && port <= 65535 {
		svc.Ports = append(svc.Ports, port)
	}
}

func parseNodeManifest(dirPath, pkgPath string, svc *ServiceInfo) {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return
	}

	var pkg struct {
		Name         string            `json:"name"`
		Scripts      map[string]string `json:"scripts"`
		Dependencies map[string]string `json:"dependencies"`
		DevDeps      map[string]string `json:"devDependencies"`
		Engines      map[string]string `json:"engines"`
	}
	if err := json.Unmarshal(data, &pkg); err == nil {
		if pkg.Name != "" {
			svc.Name = pkg.Name
		}
		// Detect package manager
		if fileExists(filepath.Join(dirPath, "pnpm-lock.yaml")) {
			svc.PackageManager = "pnpm"
		} else if fileExists(filepath.Join(dirPath, "yarn.lock")) {
			svc.PackageManager = "yarn"
		} else if fileExists(filepath.Join(dirPath, "bun.lockb")) {
			svc.PackageManager = "bun"
		} else {
			svc.PackageManager = "npm"
		}

		// Detect runtime version
		if nvmrc, err := os.ReadFile(filepath.Join(dirPath, ".nvmrc")); err == nil {
			svc.RuntimeVersion = strings.TrimSpace(string(nvmrc))
		} else if nodeVer, ok := pkg.Engines["node"]; ok {
			svc.RuntimeVersion = nodeVer
		} else {
			svc.RuntimeVersion = "20-alpine"
		}

		// Detect Frameworks & Ports
		allDeps := make(map[string]string)
		for k, v := range pkg.Dependencies {
			allDeps[k] = v
		}
		for k, v := range pkg.DevDeps {
			allDeps[k] = v
		}

		if _, ok := allDeps["next"]; ok {
			svc.Framework = "Next.js"
			addPortIfMissing(svc, 3000)
		} else if _, ok := allDeps["@nestjs/core"]; ok {
			svc.Framework = "NestJS"
			addPortIfMissing(svc, 3000)
		} else if _, ok := allDeps["express"]; ok {
			svc.Framework = "Express"
			addPortIfMissing(svc, 3000)
		} else if _, ok := allDeps["fastify"]; ok {
			svc.Framework = "Fastify"
			addPortIfMissing(svc, 3000)
		} else if _, ok := allDeps["vue"]; ok || allDeps["nuxt"] != "" {
			svc.Framework = "Nuxt / Vue"
			addPortIfMissing(svc, 3000)
		} else if _, ok := allDeps["react"]; ok {
			svc.Framework = "React"
			addPortIfMissing(svc, 3000)
		}

		// Commands
		if _, ok := pkg.Scripts["build"]; ok {
			switch svc.PackageManager {
			case "pnpm":
				svc.BuildCommand = "pnpm run build"
			case "yarn":
				svc.BuildCommand = "yarn build"
			case "bun":
				svc.BuildCommand = "bun run build"
			default:
				svc.BuildCommand = "npm run build"
			}
		}

		if _, ok := pkg.Scripts["start"]; ok {
			switch svc.PackageManager {
			case "pnpm":
				svc.StartCommand = "pnpm start"
			case "yarn":
				svc.StartCommand = "yarn start"
			case "bun":
				svc.StartCommand = "bun start"
			default:
				svc.StartCommand = "npm start"
			}
		} else if _, ok := pkg.Scripts["dev"]; ok {
			switch svc.PackageManager {
			case "pnpm":
				svc.StartCommand = "pnpm run dev"
			case "yarn":
				svc.StartCommand = "yarn dev"
			default:
				svc.StartCommand = "npm run dev"
			}
		}
	}
}

func parsePythonManifest(dirPath string, svc *ServiceInfo) {
	svc.PackageManager = "pip"
	if fileExists(filepath.Join(dirPath, "poetry.lock")) {
		svc.PackageManager = "poetry"
	} else if fileExists(filepath.Join(dirPath, "Pipfile.lock")) {
		svc.PackageManager = "pipenv"
	}

	svc.RuntimeVersion = "3.11-slim"

	// Check python-version or pyproject
	if pyVer, err := os.ReadFile(filepath.Join(dirPath, ".python-version")); err == nil {
		svc.RuntimeVersion = strings.TrimSpace(string(pyVer))
	}

	content := ""
	for _, f := range []string{"requirements.txt", "pyproject.toml", "Pipfile"} {
		if data, err := os.ReadFile(filepath.Join(dirPath, f)); err == nil {
			content += string(data) + "\n"
		}
	}
	contentLower := strings.ToLower(content)

	if strings.Contains(contentLower, "fastapi") {
		svc.Framework = "FastAPI"
		addPortIfMissing(svc, 8000)
		svc.StartCommand = "uvicorn main:app --host 0.0.0.0 --port 8000"
	} else if strings.Contains(contentLower, "django") {
		svc.Framework = "Django"
		addPortIfMissing(svc, 8000)
		svc.StartCommand = "python manage.py runserver 0.0.0.0:8000"
	} else if strings.Contains(contentLower, "flask") {
		svc.Framework = "Flask"
		addPortIfMissing(svc, 5000)
		svc.StartCommand = "python -m flask run --host=0.0.0.0 --port=5000"
	} else if strings.Contains(contentLower, "streamlit") {
		svc.Framework = "Streamlit"
		addPortIfMissing(svc, 8501)
		svc.StartCommand = "streamlit run app.py --server.port=8501 --server.address=0.0.0.0"
	} else {
		// Generic script
		if fileExists(filepath.Join(dirPath, "app.py")) {
			svc.StartCommand = "python app.py"
		} else if fileExists(filepath.Join(dirPath, "main.py")) {
			svc.StartCommand = "python main.py"
		}
	}
}

func parseGoManifest(goModPath string, svc *ServiceInfo) {
	svc.PackageManager = "go mod"
	dirPath := filepath.Dir(goModPath)

	// Determine entrypoint: check root or cmd/*
	binaryName := svc.Name
	if binaryName == "" || binaryName == "." {
		binaryName = "app"
	}
	buildTarget := "."

	if !fileExists(filepath.Join(dirPath, "main.go")) {
		// Look in cmd/
		cmdDir := filepath.Join(dirPath, "cmd")
		if entries, err := os.ReadDir(cmdDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					buildTarget = "./cmd/" + e.Name()
					binaryName = e.Name()
					break
				}
			}
		}
	}

	svc.BuildCommand = fmt.Sprintf("CGO_ENABLED=0 GOOS=linux go build -ldflags=\"-w -s\" -o /app/%s %s", binaryName, buildTarget)
	svc.StartCommand = fmt.Sprintf("/app/%s", binaryName)

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	isWebService := false
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "go ") {
			svc.RuntimeVersion = strings.TrimSpace(strings.TrimPrefix(l, "go ")) + "-alpine"
		}
		if strings.Contains(l, "gin-gonic/gin") {
			svc.Framework = "Gin"
			addPortIfMissing(svc, 8080)
			isWebService = true
		} else if strings.Contains(l, "gofiber/fiber") {
			svc.Framework = "Fiber"
			addPortIfMissing(svc, 3000)
			isWebService = true
		} else if strings.Contains(l, "labstack/echo") {
			svc.Framework = "Echo"
			addPortIfMissing(svc, 8080)
			isWebService = true
		} else if strings.Contains(l, "go-chi/chi") {
			svc.Framework = "Chi"
			addPortIfMissing(svc, 8080)
			isWebService = true
		} else if strings.Contains(l, "spf13/cobra") {
			svc.Framework = "Cobra CLI"
		} else if strings.Contains(l, "urfave/cli") {
			svc.Framework = "CLI"
		}
	}
	if svc.RuntimeVersion == "" {
		svc.RuntimeVersion = "1.22-alpine"
	}

	// Only assign default port if it is confirmed to be a web service or source has port mentions
	if isWebService && len(svc.Ports) == 0 {
		addPortIfMissing(svc, 8080)
	}
}

func parseRustManifest(cargoPath string, svc *ServiceInfo) {
	svc.PackageManager = "cargo"
	svc.RuntimeVersion = "alpine"
	svc.BuildCommand = "cargo build --release"

	data, err := os.ReadFile(cargoPath)
	if err == nil {
		content := string(data)
		if strings.Contains(content, "actix-web") {
			svc.Framework = "Actix Web"
			addPortIfMissing(svc, 8080)
		} else if strings.Contains(content, "axum") {
			svc.Framework = "Axum"
			addPortIfMissing(svc, 3000)
		} else if strings.Contains(content, "rocket") {
			svc.Framework = "Rocket"
			addPortIfMissing(svc, 8000)
		}
	}
	if len(svc.Ports) == 0 {
		addPortIfMissing(svc, 8080)
	}
}

func parseJavaManifest(dirPath, pomPath, gradlePath, gradleKtsPath string, svc *ServiceInfo) {
	svc.RuntimeVersion = "17-jdk-slim"
	if fileExists(pomPath) {
		svc.PackageManager = "maven"
		svc.BuildCommand = "./mvnw clean package -DskipTests"
		svc.StartCommand = "java -jar target/*.jar"
	} else {
		svc.PackageManager = "gradle"
		svc.BuildCommand = "./gradlew build -x test"
		svc.StartCommand = "java -jar build/libs/*.jar"
	}

	// Read pom.xml or gradle
	data, _ := os.ReadFile(pomPath)
	if len(data) == 0 {
		data, _ = os.ReadFile(gradlePath)
	}
	content := string(data)
	if strings.Contains(content, "spring-boot") {
		svc.Framework = "Spring Boot"
		addPortIfMissing(svc, 8080)
	} else if strings.Contains(content, "quarkus") {
		svc.Framework = "Quarkus"
		addPortIfMissing(svc, 8080)
	} else {
		addPortIfMissing(svc, 8080)
	}
}

func parsePHPManifest(composerPath string, svc *ServiceInfo) {
	svc.PackageManager = "composer"
	svc.RuntimeVersion = "8.2-fpm-alpine"
	data, err := os.ReadFile(composerPath)
	if err == nil {
		content := string(data)
		if strings.Contains(content, "laravel/framework") {
			svc.Framework = "Laravel"
			svc.StartCommand = "php artisan serve --host=0.0.0.0 --port=8000"
			addPortIfMissing(svc, 8000)
		} else if strings.Contains(content, "symfony/framework-bundle") {
			svc.Framework = "Symfony"
			svc.StartCommand = "php -S 0.0.0.0:8000 -t public"
			addPortIfMissing(svc, 8000)
		}
	}
	if len(svc.Ports) == 0 {
		addPortIfMissing(svc, 8000)
	}
}

func parseRubyManifest(gemfilePath string, svc *ServiceInfo) {
	svc.PackageManager = "bundler"
	svc.RuntimeVersion = "3.2-slim"
	data, err := os.ReadFile(gemfilePath)
	if err == nil {
		content := string(data)
		if strings.Contains(content, "rails") {
			svc.Framework = "Ruby on Rails"
			svc.StartCommand = "bin/rails server -b 0.0.0.0 -p 3000"
			addPortIfMissing(svc, 3000)
		} else if strings.Contains(content, "sinatra") {
			svc.Framework = "Sinatra"
			svc.StartCommand = "ruby app.rb -o 0.0.0.0 -p 4567"
			addPortIfMissing(svc, 4567)
		}
	}
	if len(svc.Ports) == 0 {
		addPortIfMissing(svc, 3000)
	}
}

func detectPortsFromSource(dirPath string, svc *ServiceInfo) {
	// Scan common entrypoint files for port references
	candidates := []string{
		"server.js", "app.js", "index.js", "main.js", "src/index.ts", "src/main.ts", "src/server.ts",
		"main.go", "cmd/main.go", "app.py", "main.py", "server.py",
	}

	for _, cand := range candidates {
		fullPath := filepath.Join(dirPath, cand)
		f, err := os.Open(fullPath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			matches := portRegex.FindStringSubmatch(scanner.Text())
			if len(matches) > 1 {
				if p, err := strconv.Atoi(matches[1]); err == nil && p >= 1000 && p <= 65535 {
					addPortIfMissing(svc, p)
					break
				}
			}
		}
		f.Close()
		if len(svc.Ports) > 0 {
			break
		}
	}
}
