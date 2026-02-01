package enhanced

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// PathBruter checks for common paths and backup files.
type PathBruter struct {
	client      *http.Client
	userAgent   string
	concurrency int
}

// PathResult represents a discovered path.
type PathResult struct {
	Path         string
	URL          string
	StatusCode   int
	ContentType  string
	ContentLength int64
	Category     string
	Interesting  bool
}

// NewPathBruter creates a new path bruter.
func NewPathBruter(userAgent string, concurrency int) *PathBruter {
	if concurrency <= 0 {
		concurrency = 10
	}
	return &PathBruter{
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		},
		userAgent:   userAgent,
		concurrency: concurrency,
	}
}

// CommonPaths returns a list of common paths to check.
func (p *PathBruter) CommonPaths() []string {
	return []string{
		// Admin panels
		"/admin", "/admin/", "/administrator", "/admin.php", "/admin.html",
		"/wp-admin", "/wp-admin/", "/wp-login.php",
		"/cpanel", "/phpmyadmin", "/pma", "/adminer.php",
		"/manager", "/management", "/dashboard",
		"/panel", "/controlpanel", "/admin-console",

		// API endpoints
		"/api", "/api/", "/api/v1", "/api/v2", "/api/v3",
		"/rest", "/rest/", "/graphql", "/graphiql",
		"/swagger", "/swagger-ui", "/swagger.json", "/swagger.yaml",
		"/openapi.json", "/openapi.yaml", "/api-docs",
		"/v1", "/v2", "/v3",

		// Config & Debug
		"/.env", "/config.php", "/config.json", "/config.yml",
		"/settings.php", "/settings.json", "/configuration.php",
		"/debug", "/debug/", "/phpinfo.php", "/info.php",
		"/server-status", "/server-info",
		"/.git", "/.git/config", "/.git/HEAD",
		"/.svn", "/.svn/entries",
		"/.hg", "/.bzr",
		"/web.config", "/crossdomain.xml",
		"/.htaccess", "/.htpasswd",
		"/composer.json", "/composer.lock",
		"/package.json", "/package-lock.json", "/yarn.lock",
		"/Gemfile", "/Gemfile.lock",
		"/requirements.txt", "/Pipfile", "/Pipfile.lock",
		"/Dockerfile", "/docker-compose.yml", "/docker-compose.yaml",
		"/.dockerignore", "/.gitignore",
		"/Makefile", "/Rakefile", "/Gruntfile.js", "/gulpfile.js",
		"/webpack.config.js", "/tsconfig.json", "/babel.config.js",

		// Backup files
		"/backup", "/backup/", "/backups", "/backups/",
		"/db", "/database", "/dump", "/sql",
		"/backup.sql", "/backup.zip", "/backup.tar.gz",
		"/db.sql", "/database.sql", "/dump.sql",
		"/site.zip", "/www.zip", "/web.zip",
		"/.backup", "/old", "/old/",

		// Common files
		"/robots.txt", "/sitemap.xml", "/sitemap_index.xml",
		"/crossdomain.xml", "/clientaccesspolicy.xml",
		"/humans.txt", "/security.txt", "/.well-known/security.txt",
		"/favicon.ico", "/apple-touch-icon.png",
		"/manifest.json", "/browserconfig.xml",
		"/service-worker.js", "/sw.js",

		// CMS specific
		"/wp-content", "/wp-includes", "/wp-json", "/wp-json/wp/v2",
		"/xmlrpc.php", "/wp-cron.php", "/wp-config.php.bak",
		"/joomla", "/components", "/modules", "/plugins",
		"/administrator/index.php",
		"/drupal", "/sites/default", "/core",
		"/magento", "/downloader", "/app/etc/local.xml",

		// Java/Spring
		"/actuator", "/actuator/health", "/actuator/info",
		"/actuator/env", "/actuator/beans", "/actuator/mappings",
		"/jolokia", "/console", "/h2-console",
		"/WEB-INF/web.xml", "/META-INF/MANIFEST.MF",

		// Node.js/Express
		"/node_modules", "/.npm", "/npm-debug.log",
		"/server.js", "/app.js", "/index.js",

		// AWS/Cloud
		"/.aws/credentials", "/.aws/config",
		"/metadata", "/latest/meta-data",
		"/.firebase", "/firebase.json",

		// Other
		"/test", "/testing", "/dev", "/development",
		"/staging", "/stage", "/uat",
		"/demo", "/sample", "/example",
		"/temp", "/tmp", "/cache",
		"/log", "/logs", "/error.log", "/access.log",
		"/upload", "/uploads", "/files", "/documents",
		"/images", "/img", "/assets", "/static",
		"/include", "/includes", "/lib", "/library",
		"/cgi-bin", "/scripts", "/bin",
		"/console", "/shell", "/terminal",
		"/login", "/signin", "/signup", "/register",
		"/logout", "/signout", "/auth", "/oauth",
		"/user", "/users", "/account", "/profile",
		"/status", "/health", "/healthcheck", "/ping",
		"/version", "/info", "/about",
		"/search", "/query", "/find",
		"/download", "/export", "/report",
	}
}

// BackupExtensions returns file extensions to check for backups.
func (p *PathBruter) BackupExtensions() []string {
	return []string{
		".bak", ".backup", ".old", ".orig", ".original",
		".save", ".saved", ".copy", ".tmp", ".temp",
		".swp", ".swo", "~", ".bkp", ".bk",
		".1", ".2", "_backup", "_old", "_bak",
		".zip", ".tar", ".tar.gz", ".tgz", ".rar", ".7z",
		".sql", ".sql.gz", ".sql.bak",
		".log", ".txt", ".conf", ".config",
	}
}

// Brute checks common paths against a target.
func (p *PathBruter) Brute(targetURL string) ([]PathResult, error) {
	baseURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	paths := p.CommonPaths()
	results := make([]PathResult, 0)
	resultChan := make(chan PathResult, len(paths))
	pathChan := make(chan string, len(paths))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathChan {
				checkURL := baseURL.Scheme + "://" + baseURL.Host + path
				if result := p.checkPath(checkURL, path); result != nil {
					resultChan <- *result
				}
			}
		}()
	}

	// Send paths to workers
	go func() {
		for _, path := range paths {
			pathChan <- path
		}
		close(pathChan)
	}()

	// Wait for workers and close result channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	return results, nil
}

// BruteBackups checks for backup files of known files.
func (p *PathBruter) BruteBackups(knownFiles []string) ([]PathResult, error) {
	extensions := p.BackupExtensions()
	results := make([]PathResult, 0)
	resultChan := make(chan PathResult, len(knownFiles)*len(extensions))
	pathChan := make(chan string, len(knownFiles)*len(extensions))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for checkURL := range pathChan {
				path := strings.TrimPrefix(checkURL, "http")
				if idx := strings.Index(path, "/"); idx != -1 {
					path = path[idx:]
				}
				if result := p.checkPath(checkURL, path); result != nil {
					result.Category = "backup"
					result.Interesting = true
					resultChan <- *result
				}
			}
		}()
	}

	// Generate backup URL variations
	go func() {
		for _, fileURL := range knownFiles {
			parsedURL, err := url.Parse(fileURL)
			if err != nil {
				continue
			}

			path := parsedURL.Path
			base := parsedURL.Scheme + "://" + parsedURL.Host

			for _, ext := range extensions {
				// file.ext.bak
				pathChan <- base + path + ext

				// file.bak.ext (for files with extensions)
				if idx := strings.LastIndex(path, "."); idx != -1 {
					pathChan <- base + path[:idx] + ext + path[idx:]
				}
			}
		}
		close(pathChan)
	}()

	// Wait and close
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		results = append(results, result)
	}

	return results, nil
}

// checkPath checks if a path exists and returns relevant info.
func (p *PathBruter) checkPath(targetURL, path string) *PathResult {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	// Only interested in 200, 301, 302, 401, 403
	interesting := resp.StatusCode == 200 ||
		resp.StatusCode == 301 ||
		resp.StatusCode == 302 ||
		resp.StatusCode == 401 ||
		resp.StatusCode == 403

	if !interesting {
		return nil
	}

	result := &PathResult{
		Path:          path,
		URL:           targetURL,
		StatusCode:    resp.StatusCode,
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.ContentLength,
		Category:      p.categorize(path),
		Interesting:   p.isInteresting(path, resp.StatusCode, resp.Header.Get("Content-Type")),
	}

	return result
}

// categorize categorizes a path based on its name.
func (p *PathBruter) categorize(path string) string {
	lowerPath := strings.ToLower(path)

	switch {
	case strings.Contains(lowerPath, "admin"):
		return "admin"
	case strings.Contains(lowerPath, "api") || strings.Contains(lowerPath, "rest") || strings.Contains(lowerPath, "graphql"):
		return "api"
	case strings.Contains(lowerPath, "config") || strings.Contains(lowerPath, "env") || strings.Contains(lowerPath, "settings"):
		return "config"
	case strings.Contains(lowerPath, ".git") || strings.Contains(lowerPath, ".svn") || strings.Contains(lowerPath, ".hg"):
		return "vcs"
	case strings.Contains(lowerPath, "backup") || strings.Contains(lowerPath, ".bak") || strings.Contains(lowerPath, ".old"):
		return "backup"
	case strings.Contains(lowerPath, "debug") || strings.Contains(lowerPath, "phpinfo") || strings.Contains(lowerPath, "actuator"):
		return "debug"
	case strings.Contains(lowerPath, "swagger") || strings.Contains(lowerPath, "openapi") || strings.Contains(lowerPath, "api-docs"):
		return "documentation"
	case strings.Contains(lowerPath, "log") || strings.Contains(lowerPath, "error"):
		return "logs"
	case strings.Contains(lowerPath, "upload") || strings.Contains(lowerPath, "file"):
		return "uploads"
	case strings.Contains(lowerPath, "wp-") || strings.Contains(lowerPath, "joomla") || strings.Contains(lowerPath, "drupal"):
		return "cms"
	default:
		return "other"
	}
}

// isInteresting determines if a finding is particularly interesting.
func (p *PathBruter) isInteresting(path string, statusCode int, contentType string) bool {
	lowerPath := strings.ToLower(path)

	// High-value targets
	highValue := []string{
		".env", ".git", "config.php", "wp-config", "settings.py",
		"credentials", "secret", "password", ".htpasswd",
		"backup", "dump.sql", "database.sql",
		"actuator", "phpinfo", "debug",
		"swagger", "graphql", "api-docs",
	}

	for _, hv := range highValue {
		if strings.Contains(lowerPath, hv) {
			return true
		}
	}

	// 401/403 on admin paths is interesting
	if (statusCode == 401 || statusCode == 403) &&
		(strings.Contains(lowerPath, "admin") || strings.Contains(lowerPath, "management")) {
		return true
	}

	// Source code exposure
	if statusCode == 200 && strings.Contains(contentType, "text/plain") {
		codeExts := []string{".php", ".py", ".rb", ".java", ".go", ".js"}
		for _, ext := range codeExts {
			if strings.HasSuffix(lowerPath, ext) {
				return true
			}
		}
	}

	return false
}
