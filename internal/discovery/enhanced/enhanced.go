// Package enhanced provides advanced discovery capabilities for web application crawling.
package enhanced

import (
	"net/http"
	"net/url"
	"sync"
)

// EnhancedDiscovery orchestrates all enhanced discovery modules.
type EnhancedDiscovery struct {
	userAgent    string
	concurrency  int
	robots       *RobotsParser
	sitemap      *SitemapParser
	sourceMap    *SourceMapParser
	pathBruter   *PathBruter
	fingerprint  *TechFingerprinter
	paramDiscov  *ParameterDiscovery
	jsExtractor  *JSExtractor
}

// Config holds configuration for enhanced discovery.
type Config struct {
	UserAgent   string
	Concurrency int
	EnableAll   bool

	// Individual module toggles
	EnableRobots      bool
	EnableSitemap     bool
	EnableSourceMaps  bool
	EnablePathBrute   bool
	EnableFingerprint bool
	EnableParamDiscov bool
	EnableJSExtract   bool
}

// DiscoveryResult contains all discovery results.
type DiscoveryResult struct {
	Target string

	// Robots.txt results
	RobotsResult *RobotsResult

	// Sitemap results
	SitemapURLs []SitemapURL

	// Source map results
	SourceMapResults []*SourceMapResult

	// Path brute results
	PathResults []PathResult

	// Technology fingerprint
	TechResult *TechResult

	// Parameter discovery
	ParameterResult *ParameterResult

	// JS extraction results
	JSResults []JSExtractionResult

	// Aggregated discoveries
	AllURLs         []string
	AllAPIEndpoints []string
	AllRoutes       []string
	AllSecrets      []SecretFinding
	AllParameters   []Parameter
}

// NewEnhancedDiscovery creates a new enhanced discovery orchestrator.
func NewEnhancedDiscovery(cfg Config) *EnhancedDiscovery {
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Mozilla/5.0 (compatible; DAST-Crawler/1.0)"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}

	ed := &EnhancedDiscovery{
		userAgent:   cfg.UserAgent,
		concurrency: cfg.Concurrency,
	}

	// Initialize modules based on config
	if cfg.EnableAll || cfg.EnableRobots {
		ed.robots = NewRobotsParser(cfg.UserAgent)
	}
	if cfg.EnableAll || cfg.EnableSitemap {
		ed.sitemap = NewSitemapParser(cfg.UserAgent)
	}
	if cfg.EnableAll || cfg.EnableSourceMaps {
		ed.sourceMap = NewSourceMapParser(cfg.UserAgent)
	}
	if cfg.EnableAll || cfg.EnablePathBrute {
		ed.pathBruter = NewPathBruter(cfg.UserAgent, cfg.Concurrency)
	}
	if cfg.EnableAll || cfg.EnableFingerprint {
		ed.fingerprint = NewTechFingerprinter(cfg.UserAgent)
	}
	if cfg.EnableAll || cfg.EnableParamDiscov {
		ed.paramDiscov = NewParameterDiscovery()
	}
	if cfg.EnableAll || cfg.EnableJSExtract {
		ed.jsExtractor = NewJSExtractor(cfg.UserAgent, cfg.Concurrency)
	}

	return ed
}

// Discover performs all enabled discovery operations on a target.
func (ed *EnhancedDiscovery) Discover(targetURL string, headers http.Header, htmlContent string, cookies []*http.Cookie, jsURLs []string, knownURLs []string) *DiscoveryResult {
	result := &DiscoveryResult{
		Target:          targetURL,
		AllURLs:         make([]string, 0),
		AllAPIEndpoints: make([]string, 0),
		AllRoutes:       make([]string, 0),
		AllSecrets:      make([]SecretFinding, 0),
		AllParameters:   make([]Parameter, 0),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Robots.txt parsing
	if ed.robots != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if robotsResult, err := ed.robots.Parse(targetURL); err == nil && robotsResult != nil {
				mu.Lock()
				result.RobotsResult = robotsResult
				// Add interesting paths to URLs
				for _, path := range ed.robots.GetInterestingPaths(robotsResult) {
					baseURL, _ := url.Parse(targetURL)
					if baseURL != nil {
						fullURL := baseURL.Scheme + "://" + baseURL.Host + path
						result.AllURLs = append(result.AllURLs, fullURL)
					}
				}
				// Add sitemaps
				for _, sitemapURL := range robotsResult.Sitemaps {
					result.AllURLs = append(result.AllURLs, sitemapURL)
				}
				mu.Unlock()
			}
		}()
	}

	// Sitemap parsing
	if ed.sitemap != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sitemapURLs, err := ed.sitemap.Discover(targetURL); err == nil && len(sitemapURLs) > 0 {
				mu.Lock()
				result.SitemapURLs = sitemapURLs
				for _, su := range sitemapURLs {
					result.AllURLs = append(result.AllURLs, su.Loc)
				}
				mu.Unlock()
			}
		}()
	}

	// Source map parsing
	if ed.sourceMap != nil && len(jsURLs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sourceMaps := ed.sourceMap.FindSourceMaps(jsURLs)
			results := make([]*SourceMapResult, 0)
			for _, smURL := range sourceMaps {
				if smResult, err := ed.sourceMap.Parse(smURL); err == nil && smResult != nil {
					results = append(results, smResult)
				}
			}
			mu.Lock()
			result.SourceMapResults = results
			for _, smResult := range results {
				result.AllRoutes = append(result.AllRoutes, smResult.Routes...)
				result.AllAPIEndpoints = append(result.AllAPIEndpoints, smResult.APIEndpoints...)
				result.AllSecrets = append(result.AllSecrets, smResult.Secrets...)
			}
			mu.Unlock()
		}()
	}

	// Path brute forcing
	if ed.pathBruter != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if pathResults, err := ed.pathBruter.Brute(targetURL); err == nil {
				mu.Lock()
				result.PathResults = pathResults
				for _, pr := range pathResults {
					result.AllURLs = append(result.AllURLs, pr.URL)
					if pr.Category == "api" {
						result.AllAPIEndpoints = append(result.AllAPIEndpoints, pr.URL)
					}
				}
				mu.Unlock()
			}
		}()
	}

	// Technology fingerprinting
	if ed.fingerprint != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			techResult := ed.fingerprint.Fingerprint(headers, htmlContent, cookies)
			mu.Lock()
			result.TechResult = techResult
			mu.Unlock()
		}()
	}

	// Parameter discovery
	if ed.paramDiscov != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// From URLs
			paramResult := ed.paramDiscov.ExtractFromURLs(knownURLs)
			// From HTML
			htmlParams := ed.paramDiscov.ExtractFromHTML(htmlContent)

			mu.Lock()
			result.ParameterResult = paramResult
			result.AllParameters = append(result.AllParameters, paramResult.QueryParams...)
			result.AllParameters = append(result.AllParameters, paramResult.BodyParams...)
			result.AllParameters = append(result.AllParameters, paramResult.PathParams...)
			result.AllParameters = append(result.AllParameters, htmlParams...)
			mu.Unlock()
		}()
	}

	// JavaScript extraction
	if ed.jsExtractor != nil && len(jsURLs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jsResults := ed.jsExtractor.ExtractFromURLs(jsURLs)
			mu.Lock()
			result.JSResults = jsResults
			for _, jr := range jsResults {
				result.AllURLs = append(result.AllURLs, jr.URLs...)
				result.AllAPIEndpoints = append(result.AllAPIEndpoints, jr.APIEndpoints...)
				result.AllRoutes = append(result.AllRoutes, jr.Routes...)
				result.AllSecrets = append(result.AllSecrets, jr.Secrets...)
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Deduplicate results
	result.AllURLs = dedupe(result.AllURLs)
	result.AllAPIEndpoints = dedupe(result.AllAPIEndpoints)
	result.AllRoutes = dedupe(result.AllRoutes)

	return result
}

// DiscoverQuick performs a quick discovery with essential modules only.
func (ed *EnhancedDiscovery) DiscoverQuick(targetURL string) *DiscoveryResult {
	result := &DiscoveryResult{
		Target:          targetURL,
		AllURLs:         make([]string, 0),
		AllAPIEndpoints: make([]string, 0),
		AllRoutes:       make([]string, 0),
		AllSecrets:      make([]SecretFinding, 0),
		AllParameters:   make([]Parameter, 0),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Only run robots and sitemap
	if ed.robots != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if robotsResult, err := ed.robots.Parse(targetURL); err == nil && robotsResult != nil {
				mu.Lock()
				result.RobotsResult = robotsResult
				for _, path := range ed.robots.GetInterestingPaths(robotsResult) {
					baseURL, _ := url.Parse(targetURL)
					if baseURL != nil {
						fullURL := baseURL.Scheme + "://" + baseURL.Host + path
						result.AllURLs = append(result.AllURLs, fullURL)
					}
				}
				mu.Unlock()
			}
		}()
	}

	if ed.sitemap != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sitemapURLs, err := ed.sitemap.Discover(targetURL); err == nil && len(sitemapURLs) > 0 {
				mu.Lock()
				result.SitemapURLs = sitemapURLs
				for _, su := range sitemapURLs {
					result.AllURLs = append(result.AllURLs, su.Loc)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	result.AllURLs = dedupe(result.AllURLs)
	return result
}

// dedupe removes duplicates from a string slice.
func dedupe(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// GetRobotsParser returns the robots parser for direct use.
func (ed *EnhancedDiscovery) GetRobotsParser() *RobotsParser {
	return ed.robots
}

// GetSitemapParser returns the sitemap parser for direct use.
func (ed *EnhancedDiscovery) GetSitemapParser() *SitemapParser {
	return ed.sitemap
}

// GetSourceMapParser returns the source map parser for direct use.
func (ed *EnhancedDiscovery) GetSourceMapParser() *SourceMapParser {
	return ed.sourceMap
}

// GetPathBruter returns the path bruter for direct use.
func (ed *EnhancedDiscovery) GetPathBruter() *PathBruter {
	return ed.pathBruter
}

// GetFingerprinter returns the fingerprinter for direct use.
func (ed *EnhancedDiscovery) GetFingerprinter() *TechFingerprinter {
	return ed.fingerprint
}

// GetParameterDiscovery returns the parameter discovery for direct use.
func (ed *EnhancedDiscovery) GetParameterDiscovery() *ParameterDiscovery {
	return ed.paramDiscov
}

// GetJSExtractor returns the JS extractor for direct use.
func (ed *EnhancedDiscovery) GetJSExtractor() *JSExtractor {
	return ed.jsExtractor
}
