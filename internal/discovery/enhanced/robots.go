package enhanced

import (
	"bufio"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RobotsParser parses robots.txt to discover paths.
type RobotsParser struct {
	client    *http.Client
	userAgent string
}

// RobotsResult contains parsed robots.txt data.
type RobotsResult struct {
	AllowedPaths    []string
	DisallowedPaths []string
	Sitemaps        []string
	CrawlDelay      int
	Host            string
	UserAgentRules  map[string]*RobotsRules
}

// RobotsRules contains rules for a specific user agent.
type RobotsRules struct {
	UserAgent  string
	Allow      []string
	Disallow   []string
	CrawlDelay int
}

// NewRobotsParser creates a new robots.txt parser.
func NewRobotsParser(userAgent string) *RobotsParser {
	return &RobotsParser{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: userAgent,
	}
}

// Parse fetches and parses robots.txt for a target.
func (p *RobotsParser) Parse(targetURL string) (*RobotsResult, error) {
	baseURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	robotsURL := baseURL.Scheme + "://" + baseURL.Host + "/robots.txt"

	req, err := http.NewRequest("GET", robotsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &RobotsResult{}, nil
	}

	return p.parseContent(resp.Body)
}

// parseContent parses robots.txt content.
func (p *RobotsParser) parseContent(reader io.Reader) (*RobotsResult, error) {
	result := &RobotsResult{
		AllowedPaths:    make([]string, 0),
		DisallowedPaths: make([]string, 0),
		Sitemaps:        make([]string, 0),
		UserAgentRules:  make(map[string]*RobotsRules),
	}

	scanner := bufio.NewScanner(reader)
	var currentUA string
	var currentRules *RobotsRules

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split into directive and value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		directive := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch directive {
		case "user-agent":
			currentUA = strings.ToLower(value)
			if _, exists := result.UserAgentRules[currentUA]; !exists {
				currentRules = &RobotsRules{
					UserAgent: value,
					Allow:     make([]string, 0),
					Disallow:  make([]string, 0),
				}
				result.UserAgentRules[currentUA] = currentRules
			} else {
				currentRules = result.UserAgentRules[currentUA]
			}

		case "disallow":
			if value != "" {
				result.DisallowedPaths = append(result.DisallowedPaths, value)
				if currentRules != nil {
					currentRules.Disallow = append(currentRules.Disallow, value)
				}
			}

		case "allow":
			if value != "" {
				result.AllowedPaths = append(result.AllowedPaths, value)
				if currentRules != nil {
					currentRules.Allow = append(currentRules.Allow, value)
				}
			}

		case "sitemap":
			if value != "" {
				result.Sitemaps = append(result.Sitemaps, value)
			}

		case "host":
			result.Host = value

		case "crawl-delay":
			// Parse crawl delay
			var delay int
			if _, err := parseIntFromString(value, &delay); err == nil {
				result.CrawlDelay = delay
				if currentRules != nil {
					currentRules.CrawlDelay = delay
				}
			}
		}
	}

	return result, nil
}

// GetInterestingPaths returns paths that might reveal hidden content.
func (p *RobotsParser) GetInterestingPaths(result *RobotsResult) []string {
	interesting := make([]string, 0)
	seen := make(map[string]bool)

	// Disallowed paths are often interesting for security testing
	for _, path := range result.DisallowedPaths {
		if path == "/" || path == "" {
			continue
		}
		// Clean the path
		path = strings.TrimSuffix(path, "*")
		path = strings.TrimSuffix(path, "$")
		if path != "" && !seen[path] {
			seen[path] = true
			interesting = append(interesting, path)
		}
	}

	// Also include allowed paths
	for _, path := range result.AllowedPaths {
		if path == "/" || path == "" {
			continue
		}
		path = strings.TrimSuffix(path, "*")
		path = strings.TrimSuffix(path, "$")
		if path != "" && !seen[path] {
			seen[path] = true
			interesting = append(interesting, path)
		}
	}

	return interesting
}

// parseIntFromString helper to parse int from string
func parseIntFromString(s string, result *int) (bool, error) {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	*result = n
	return true, nil
}
