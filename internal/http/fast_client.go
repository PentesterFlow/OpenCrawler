// Package http provides a fast HTTP client for crawling non-JS pages.
package http

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/errors"
	"golang.org/x/net/html"
)

// FastClient is a high-performance HTTP client optimized for crawling.
type FastClient struct {
	client    *http.Client
	userAgent string
	headers   map[string]string
	cookies   []*http.Cookie
	retrier   *errors.Retrier
	mu        sync.RWMutex
}

// FastClientConfig holds configuration for the fast HTTP client.
type FastClientConfig struct {
	Timeout             time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	UserAgent           string
	Headers             map[string]string
	SkipTLSVerify       bool
}

// DefaultFastClientConfig returns optimized defaults.
func DefaultFastClientConfig() FastClientConfig {
	return FastClientConfig{
		Timeout:             10 * time.Second,
		MaxIdleConns:        500,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		SkipTLSVerify:       true,
	}
}

// NewFastClient creates a new high-performance HTTP client.
func NewFastClient(config FastClientConfig) *FastClient {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.SkipTLSVerify,
		},
	}

	return &FastClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   config.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		userAgent: config.UserAgent,
		headers:   config.Headers,
		retrier:   errors.NewDefaultRetrier(),
	}
}

// SetCookies sets cookies for all requests.
func (fc *FastClient) SetCookies(cookies []*http.Cookie) {
	fc.mu.Lock()
	fc.cookies = cookies
	fc.mu.Unlock()
}

// SetHeaders sets custom headers for all requests.
func (fc *FastClient) SetHeaders(headers map[string]string) {
	fc.mu.Lock()
	fc.headers = headers
	fc.mu.Unlock()
}

// FastResult contains the result of a fast HTTP request.
type FastResult struct {
	URL         string
	FinalURL    string
	StatusCode  int
	ContentType string
	HTML        string
	Title       string
	Links       []string
	Forms       []FastForm
	Scripts     []string
	Error       error
	Duration    time.Duration
}

// FastForm represents a form found in the page.
type FastForm struct {
	Action  string
	Method  string
	Inputs  []string
}

// Get performs a fast HTTP GET request and extracts links.
func (fc *FastClient) Get(ctx context.Context, targetURL string) (*FastResult, error) {
	start := time.Now()
	result := &FastResult{
		URL:   targetURL,
		Links: make([]string, 0),
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		crawlErr := errors.NewCrawlError(errors.Parse, targetURL, "request_creation", "failed to create request", err)
		result.Error = crawlErr
		return result, crawlErr
	}

	// Set headers
	req.Header.Set("User-Agent", fc.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

	fc.mu.RLock()
	for k, v := range fc.headers {
		req.Header.Set(k, v)
	}
	for _, cookie := range fc.cookies {
		req.AddCookie(cookie)
	}
	fc.mu.RUnlock()

	resp, err := fc.client.Do(req)
	if err != nil {
		// Categorize the error
		crawlErr := errors.Categorize(err, targetURL)
		result.Error = crawlErr
		return result, crawlErr
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.FinalURL = resp.Request.URL.String()
	result.ContentType = resp.Header.Get("Content-Type")

	// Check for HTTP errors
	if httpErr := errors.CategorizeHTTPStatus(resp.StatusCode, targetURL); httpErr != nil {
		result.Error = httpErr
		// Don't return error for 4xx (except auth), still process the response
		if httpErr.Type == errors.ServerError {
			return result, httpErr
		}
	}

	// Only parse HTML content
	if !strings.Contains(result.ContentType, "text/html") {
		result.Duration = time.Since(start)
		return result, nil
	}

	// Read body with limit (5MB max)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		crawlErr := errors.NewNetworkError(targetURL, "body_read", err)
		result.Error = crawlErr
		return result, crawlErr
	}
	result.HTML = string(body)

	// Parse HTML for links
	baseURL, _ := url.Parse(result.FinalURL)
	result.Links, result.Forms, result.Scripts, result.Title = fc.parseHTML(result.HTML, baseURL)

	result.Duration = time.Since(start)
	return result, nil
}

// GetWithRetry performs a GET request with automatic retries for transient errors.
func (fc *FastClient) GetWithRetry(ctx context.Context, targetURL string) (*FastResult, error) {
	var result *FastResult

	retryResult := fc.retrier.Do(ctx, "http_get", targetURL, func(ctx context.Context) error {
		var err error
		result, err = fc.Get(ctx, targetURL)
		return err
	})

	if result == nil {
		result = &FastResult{URL: targetURL}
	}

	if !retryResult.Success {
		result.Error = retryResult.LastError
		return result, retryResult.LastError
	}

	return result, nil
}

// GetBatch performs multiple GET requests concurrently.
func (fc *FastClient) GetBatch(ctx context.Context, urls []string, concurrency int) []*FastResult {
	results := make([]*FastResult, len(urls))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, u := range urls {
		wg.Add(1)
		go func(idx int, targetURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, _ := fc.Get(ctx, targetURL)
			results[idx] = result
		}(i, u)
	}

	wg.Wait()
	return results
}

// parseHTML extracts links, forms, scripts, and title from HTML.
func (fc *FastClient) parseHTML(htmlContent string, baseURL *url.URL) ([]string, []FastForm, []string, string) {
	links := make([]string, 0, 100)
	forms := make([]FastForm, 0, 10)
	scripts := make([]string, 0, 20)
	title := ""
	seen := make(map[string]bool)

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return links, forms, scripts, title
	}

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a":
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						link := fc.resolveURL(attr.Val, baseURL)
						if link != "" && !seen[link] {
							seen[link] = true
							links = append(links, link)
						}
						break
					}
				}
			case "form":
				form := FastForm{Method: "GET"}
				for _, attr := range n.Attr {
					switch attr.Key {
					case "action":
						form.Action = fc.resolveURL(attr.Val, baseURL)
					case "method":
						form.Method = strings.ToUpper(attr.Val)
					}
				}
				// Extract inputs
				form.Inputs = fc.extractFormInputs(n)
				forms = append(forms, form)
			case "script":
				for _, attr := range n.Attr {
					if attr.Key == "src" {
						src := fc.resolveURL(attr.Val, baseURL)
						if src != "" {
							scripts = append(scripts, src)
						}
						break
					}
				}
			case "title":
				if n.FirstChild != nil {
					title = n.FirstChild.Data
				}
			case "link":
				// Extract CSS/preload links
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						link := fc.resolveURL(attr.Val, baseURL)
						if link != "" && !seen[link] {
							seen[link] = true
							// Only add HTML-like links, not CSS
							if !strings.HasSuffix(link, ".css") {
								links = append(links, link)
							}
						}
						break
					}
				}
			case "area", "base":
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						link := fc.resolveURL(attr.Val, baseURL)
						if link != "" && !seen[link] {
							seen[link] = true
							links = append(links, link)
						}
						break
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return links, forms, scripts, title
}

// extractFormInputs extracts input names from a form node.
func (fc *FastClient) extractFormInputs(formNode *html.Node) []string {
	inputs := make([]string, 0, 10)

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "input" || n.Data == "textarea" || n.Data == "select" {
				for _, attr := range n.Attr {
					if attr.Key == "name" && attr.Val != "" {
						inputs = append(inputs, attr.Val)
						break
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(formNode)
	return inputs
}

// resolveURL resolves a relative URL against a base URL.
func (fc *FastClient) resolveURL(href string, baseURL *url.URL) string {
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") ||
		strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") ||
		strings.HasPrefix(href, "data:") {
		return ""
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}

	resolved := baseURL.ResolveReference(parsed)

	// Only return HTTP(S) URLs
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}

	return resolved.String()
}

// Head performs a fast HTTP HEAD request.
func (fc *FastClient) Head(ctx context.Context, targetURL string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", targetURL, nil)
	if err != nil {
		return 0, "", errors.NewCrawlError(errors.Parse, targetURL, "request_creation", "failed to create request", err)
	}

	req.Header.Set("User-Agent", fc.userAgent)

	fc.mu.RLock()
	for k, v := range fc.headers {
		req.Header.Set(k, v)
	}
	fc.mu.RUnlock()

	resp, err := fc.client.Do(req)
	if err != nil {
		return 0, "", errors.Categorize(err, targetURL)
	}
	resp.Body.Close()

	return resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

// SetRetryConfig sets custom retry configuration.
func (fc *FastClient) SetRetryConfig(config errors.RetryConfig) {
	fc.retrier = errors.NewRetrier(config)
}

// IsRetryableError checks if an error should trigger a retry.
func (fc *FastClient) IsRetryableError(err error) bool {
	return errors.IsRetryable(err)
}

// Close closes the HTTP client.
func (fc *FastClient) Close() {
	fc.client.CloseIdleConnections()
}
