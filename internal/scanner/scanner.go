package scanner

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var defaultIgnoredDirs = map[string]bool{
	".git":         true,
	".github":      false, // allow scanning CI files
	".gitlab":      false,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".idea":        true,
	".vscode":      true,
	"target":       true,
	"bin":          true,
	"obj":          true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".nuxt":        true,
	".cache":       true,
	".terraform":   true,
}

var secretFilePatterns = []string{
	".pem", ".key", ".pfx", ".p12", ".pkcs12", "id_rsa", "id_ed25519", "credentials", "secrets.json",
}

// Scanner performs recursive project analysis.
type Scanner struct {
	rootDir string
}

func NewScanner(rootDir string) *Scanner {
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		absPath = rootDir
	}
	return &Scanner{rootDir: absPath}
}

// Scan examines the target directory and returns a structured ProjectInfo.
func (s *Scanner) Scan() (*ProjectInfo, error) {
	info := &ProjectInfo{
		RootPath:          s.rootDir,
		ProjectName:       filepath.Base(s.rootDir),
		Services:          make([]ServiceInfo, 0),
		ExternalServices:  make([]string, 0),
		EnvVars:           make([]string, 0),
		ExistingArtifacts: make([]string, 0),
	}

	gitIgnorePatterns := s.loadGitIgnore()

	var fileList []string
	totalScanned := 0

	err := filepath.WalkDir(s.rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		rel, err := filepath.Rel(s.rootDir, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)

		// Check directory exclusion
		if d.IsDir() {
			base := d.Name()
			if ignore, ok := defaultIgnoredDirs[base]; ok && ignore {
				return filepath.SkipDir
			}
			if matchesAnyPattern(rel, gitIgnorePatterns) {
				return filepath.SkipDir
			}
			return nil
		}

		totalScanned++

		// Check file exclusion from tree
		if matchesAnyPattern(rel, gitIgnorePatterns) {
			return nil
		}

		// Detect existing artifacts
		lowerRel := strings.ToLower(rel)
		if lowerRel == "dockerfile" || strings.HasPrefix(lowerRel, "dockerfile.") {
			info.ExistingArtifacts = appendUnique(info.ExistingArtifacts, rel)
		} else if lowerRel == ".dockerignore" {
			info.ExistingArtifacts = appendUnique(info.ExistingArtifacts, rel)
		} else if lowerRel == "docker-compose.yml" || lowerRel == "docker-compose.yaml" || lowerRel == "compose.yml" || lowerRel == "compose.yaml" {
			info.ExistingArtifacts = appendUnique(info.ExistingArtifacts, rel)
		} else if strings.HasPrefix(lowerRel, ".github/workflows/") {
			info.ExistingArtifacts = appendUnique(info.ExistingArtifacts, rel)
		} else if lowerRel == ".gitlab-ci.yml" {
			info.ExistingArtifacts = appendUnique(info.ExistingArtifacts, rel)
		} else if lowerRel == "dockship.md" {
			info.ExistingArtifacts = appendUnique(info.ExistingArtifacts, rel)
		}

		// Check for env vars (without extracting secret values)
		if strings.HasPrefix(d.Name(), ".env") {
			s.extractEnvVarKeys(path, info)
		}

		// Keep representative file tree (limit tree size to prevent token blowup)
		if len(fileList) < 120 {
			fileList = append(fileList, rel)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan project directory: %w", err)
	}

	info.TotalFilesScanned = totalScanned

	// Build compact tree representation
	sort.Strings(fileList)
	var treeBuilder strings.Builder
	for _, f := range fileList {
		treeBuilder.WriteString(f + "\n")
	}
	if totalScanned > len(fileList) {
		treeBuilder.WriteString(fmt.Sprintf("... (+%d more files)\n", totalScanned-len(fileList)))
	}
	info.FileTreeSummary = treeBuilder.String()

	// 1. Detect root service
	rootSvc := DetectService(s.rootDir, ".")
	if rootSvc != nil {
		info.Services = append(info.Services, *rootSvc)
	}

	// 2. Detect monorepo sub-services
	s.detectSubServices(info)

	if len(info.Services) > 1 {
		info.IsMonorepo = true
	}

	// 3. Detect external services (Postgres, Redis, Mongo, etc.)
	s.detectExternalServices(info)

	return info, nil
}

func (s *Scanner) detectSubServices(info *ProjectInfo) {
	// Look into common subdirs: services, packages, apps, backend, frontend, server, client, api
	commonDirs := []string{"services", "packages", "apps", "backend", "frontend", "server", "client", "api"}
	for _, dir := range commonDirs {
		dirPath := filepath.Join(s.rootDir, dir)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				subPath := filepath.Join(dirPath, entry.Name())
				relPath, _ := filepath.Rel(s.rootDir, subPath)
				svc := DetectService(subPath, filepath.ToSlash(relPath))
				if svc != nil {
					// avoid duplicate
					isDup := false
					for _, existing := range info.Services {
						if existing.Path == svc.Path {
							isDup = true
							break
						}
					}
					if !isDup {
						info.Services = append(info.Services, *svc)
					}
				}
			}
		}
	}
}

func (s *Scanner) detectExternalServices(info *ProjectInfo) {
	servicesMap := make(map[string]bool)

	checkKey := func(key string) {
		k := strings.ToUpper(key)
		if strings.Contains(k, "POSTGRES") || strings.Contains(k, "PG_") || strings.Contains(k, "DATABASE_URL") {
			servicesMap["postgres"] = true
		}
		if strings.Contains(k, "REDIS") {
			servicesMap["redis"] = true
		}
		if strings.Contains(k, "MONGO") {
			servicesMap["mongodb"] = true
		}
		if strings.Contains(k, "MYSQL") {
			servicesMap["mysql"] = true
		}
		if strings.Contains(k, "RABBITMQ") || strings.Contains(k, "AMQP") {
			servicesMap["rabbitmq"] = true
		}
		if strings.Contains(k, "KAFKA") {
			servicesMap["kafka"] = true
		}
	}

	for _, env := range info.EnvVars {
		checkKey(env)
	}

	for k := range servicesMap {
		info.ExternalServices = append(info.ExternalServices, k)
	}
	sort.Strings(info.ExternalServices)
}

// extractEnvVarKeys parses variable names only; values are strictly ignored and never stored.
func (s *Scanner) extractEnvVarKeys(filePath string, info *ProjectInfo) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) >= 1 {
			varName := strings.TrimSpace(parts[0])
			if varName != "" && !strings.Contains(varName, " ") {
				info.EnvVars = appendUnique(info.EnvVars, varName)
			}
		}
	}
}

func (s *Scanner) loadGitIgnore() []string {
	gitIgnorePath := filepath.Join(s.rootDir, ".gitignore")
	f, err := os.Open(gitIgnorePath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func matchesAnyPattern(relPath string, patterns []string) bool {
	for _, p := range patterns {
		p = strings.TrimPrefix(p, "/")
		p = strings.TrimSuffix(p, "/")
		if p == "" {
			continue
		}
		if strings.HasPrefix(relPath, p) || strings.Contains(relPath, "/"+p) || strings.HasSuffix(relPath, p) {
			return true
		}
	}
	return false
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
