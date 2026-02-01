// Package enhanced provides advanced discovery capabilities.
package enhanced

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SitemapParser parses sitemap.xml files to discover URLs.
type SitemapParser struct {
	client    *http.Client
	userAgent string
	maxDepth  int
}

// SitemapURL represents a URL entry in a sitemap.
type SitemapURL struct {
	Loc        string    `xml:"loc"`
	LastMod    string    `xml:"lastmod"`
	ChangeFreq string    `xml:"changefreq"`
	Priority   float64   `xml:"priority"`
	ParsedTime time.Time
}

// Sitemap represents a sitemap.xml structure.
type Sitemap struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []SitemapURL `xml:"url"`
}

// SitemapIndex represents a sitemap index file.
type SitemapIndex struct {
	XMLName  xml.Name        `xml:"sitemapindex"`
	Sitemaps []SitemapEntry `xml:"sitemap"`
}

// SitemapEntry represents an entry in a sitemap index.
type SitemapEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// NewSitemapParser creates a new sitemap parser.
func NewSitemapParser(userAgent string) *SitemapParser {
	return &SitemapParser{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: userAgent,
		maxDepth:  3,
	}
}

// Discover finds and parses sitemaps for a target.
func (p *SitemapParser) Discover(targetURL string) ([]SitemapURL, error) {
	baseURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	// Common sitemap locations
	sitemapPaths := []string{
		"/sitemap.xml",
		"/sitemap_index.xml",
		"/sitemap/sitemap.xml",
		"/sitemaps/sitemap.xml",
		"/sitemap1.xml",
		"/sitemap-index.xml",
		"/post-sitemap.xml",
		"/page-sitemap.xml",
		"/category-sitemap.xml",
		"/wp-sitemap.xml",
	}

	allURLs := make([]SitemapURL, 0)
	seen := make(map[string]bool)

	for _, path := range sitemapPaths {
		sitemapURL := baseURL.Scheme + "://" + baseURL.Host + path
		urls, err := p.parseSitemap(sitemapURL, 0, seen)
		if err == nil && len(urls) > 0 {
			allURLs = append(allURLs, urls...)
		}
	}

	// Also check robots.txt for sitemap references
	robotsURL := baseURL.Scheme + "://" + baseURL.Host + "/robots.txt"
	robotsSitemaps := p.findSitemapsInRobots(robotsURL)
	for _, sitemapURL := range robotsSitemaps {
		urls, err := p.parseSitemap(sitemapURL, 0, seen)
		if err == nil {
			allURLs = append(allURLs, urls...)
		}
	}

	return allURLs, nil
}

// parseSitemap fetches and parses a sitemap.
func (p *SitemapParser) parseSitemap(sitemapURL string, depth int, seen map[string]bool) ([]SitemapURL, error) {
	if depth > p.maxDepth {
		return nil, nil
	}

	if seen[sitemapURL] {
		return nil, nil
	}
	seen[sitemapURL] = true

	req, err := http.NewRequest("GET", sitemapURL, nil)
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
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Try parsing as sitemap index first
	var index SitemapIndex
	if err := xml.Unmarshal(body, &index); err == nil && len(index.Sitemaps) > 0 {
		allURLs := make([]SitemapURL, 0)
		for _, entry := range index.Sitemaps {
			urls, _ := p.parseSitemap(entry.Loc, depth+1, seen)
			allURLs = append(allURLs, urls...)
		}
		return allURLs, nil
	}

	// Try parsing as regular sitemap
	var sitemap Sitemap
	if err := xml.Unmarshal(body, &sitemap); err == nil {
		return sitemap.URLs, nil
	}

	return nil, nil
}

// findSitemapsInRobots extracts sitemap URLs from robots.txt.
func (p *SitemapParser) findSitemapsInRobots(robotsURL string) []string {
	sitemaps := make([]string, 0)

	req, err := http.NewRequest("GET", robotsURL, nil)
	if err != nil {
		return sitemaps
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return sitemaps
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return sitemaps
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sitemaps
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "sitemap:") {
			sitemap := strings.TrimSpace(line[8:])
			if sitemap != "" {
				sitemaps = append(sitemaps, sitemap)
			}
		}
	}

	return sitemaps
}

// GetURLStrings returns just the URL strings from sitemap entries.
func (p *SitemapParser) GetURLStrings(entries []SitemapURL) []string {
	urls := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Loc != "" {
			urls = append(urls, entry.Loc)
		}
	}
	return urls
}
