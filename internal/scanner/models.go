package scanner

// ServiceInfo describes an individual service detected in a project/monorepo.
type ServiceInfo struct {
	Name           string   `json:"name"`
	Path           string   `json:"path"` // relative to project root
	Language       string   `json:"language"`
	Framework      string   `json:"framework"`
	PackageManager string   `json:"package_manager"`
	RuntimeVersion string   `json:"runtime_version"`
	BuildCommand   string   `json:"build_command"`
	StartCommand   string   `json:"start_command"`
	Ports          []int    `json:"ports"`
	EnvVars        []string `json:"env_vars"`
}

// ProjectInfo contains comprehensive analysis of the inspected project directory.
type ProjectInfo struct {
	RootPath          string        `json:"root_path"`
	ProjectName       string        `json:"project_name"`
	IsMonorepo        bool          `json:"is_monorepo"`
	Services          []ServiceInfo `json:"services"`
	ExternalServices  []string      `json:"external_services"` // postgres, redis, mysql, mongodb, rabbitmq
	EnvVars           []string      `json:"env_vars"`          // Variable keys only (values redacted)
	ExistingArtifacts []string      `json:"existing_artifacts"` // Dockerfile, compose, CI files
	FileTreeSummary   string        `json:"file_tree_summary"`
	TotalFilesScanned int           `json:"total_files_scanned"`
}
