package enhanced

import (
	"net/url"
	"regexp"
	"strings"
)

// ParameterDiscovery discovers URL parameters from various sources.
type ParameterDiscovery struct {
	seen map[string]bool
}

// Parameter represents a discovered URL parameter.
type Parameter struct {
	Name     string
	Value    string
	Source   string   // URL, form, javascript, etc.
	Type     string   // query, body, header, cookie
	Context  string   // The URL or context where it was found
	Examples []string // Example values seen
}

// ParameterResult contains all discovered parameters.
type ParameterResult struct {
	QueryParams  []Parameter
	BodyParams   []Parameter
	HeaderParams []Parameter
	PathParams   []Parameter
}

// NewParameterDiscovery creates a new parameter discovery instance.
func NewParameterDiscovery() *ParameterDiscovery {
	return &ParameterDiscovery{
		seen: make(map[string]bool),
	}
}

// ExtractFromURLs extracts parameters from a list of URLs.
func (p *ParameterDiscovery) ExtractFromURLs(urls []string) *ParameterResult {
	result := &ParameterResult{
		QueryParams:  make([]Parameter, 0),
		BodyParams:   make([]Parameter, 0),
		HeaderParams: make([]Parameter, 0),
		PathParams:   make([]Parameter, 0),
	}

	paramExamples := make(map[string][]string)

	for _, rawURL := range urls {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			continue
		}

		// Extract query parameters
		for key, values := range parsedURL.Query() {
			for _, value := range values {
				if _, exists := paramExamples[key]; !exists {
					paramExamples[key] = make([]string, 0)
				}
				// Store unique examples (up to 5)
				if len(paramExamples[key]) < 5 && !contains(paramExamples[key], value) {
					paramExamples[key] = append(paramExamples[key], value)
				}
			}
		}

		// Extract path parameters (patterns like /users/123 or /api/v1/items/abc)
		p.extractPathParams(parsedURL.Path, result)
	}

	// Convert to Parameter structs
	for name, examples := range paramExamples {
		param := Parameter{
			Name:     name,
			Type:     "query",
			Source:   "url",
			Examples: examples,
		}
		if len(examples) > 0 {
			param.Value = examples[0]
		}
		result.QueryParams = append(result.QueryParams, param)
	}

	return result
}

// ExtractFromHTML extracts parameters from HTML form elements.
func (p *ParameterDiscovery) ExtractFromHTML(html string) []Parameter {
	params := make([]Parameter, 0)
	seen := make(map[string]bool)

	// Input elements
	inputRe := regexp.MustCompile(`<input[^>]+name=["']([^"']+)["'][^>]*>`)
	matches := inputRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true

			// Get type attribute
			inputType := "text"
			typeRe := regexp.MustCompile(`type=["']([^"']+)["']`)
			if typeMatch := typeRe.FindStringSubmatch(match[0]); typeMatch != nil {
				inputType = typeMatch[1]
			}

			// Get value attribute
			value := ""
			valueRe := regexp.MustCompile(`value=["']([^"']+)["']`)
			if valueMatch := valueRe.FindStringSubmatch(match[0]); valueMatch != nil {
				value = valueMatch[1]
			}

			params = append(params, Parameter{
				Name:    match[1],
				Value:   value,
				Type:    "body",
				Source:  "form-input-" + inputType,
				Context: match[0],
			})
		}
	}

	// Select elements
	selectRe := regexp.MustCompile(`<select[^>]+name=["']([^"']+)["'][^>]*>`)
	matches = selectRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			params = append(params, Parameter{
				Name:   match[1],
				Type:   "body",
				Source: "form-select",
			})
		}
	}

	// Textarea elements
	textareaRe := regexp.MustCompile(`<textarea[^>]+name=["']([^"']+)["'][^>]*>`)
	matches = textareaRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			params = append(params, Parameter{
				Name:   match[1],
				Type:   "body",
				Source: "form-textarea",
			})
		}
	}

	// Hidden inputs (often contain security tokens, IDs)
	hiddenRe := regexp.MustCompile(`<input[^>]+type=["']hidden["'][^>]+name=["']([^"']+)["'][^>]*>`)
	matches = hiddenRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) > 1 {
			// Already captured above, but mark as hidden
			for i := range params {
				if params[i].Name == match[1] {
					params[i].Source = "form-input-hidden"
				}
			}
		}
	}

	// Data attributes that might be parameters
	dataRe := regexp.MustCompile(`data-([a-zA-Z0-9_-]+)=["']([^"']*)["']`)
	matches = dataRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) > 2 {
			paramName := match[1]
			// Filter out common non-parameter data attributes
			if !isCommonDataAttr(paramName) && !seen["data-"+paramName] {
				seen["data-"+paramName] = true
				params = append(params, Parameter{
					Name:   paramName,
					Value:  match[2],
					Type:   "body",
					Source: "data-attribute",
				})
			}
		}
	}

	return params
}

// ExtractFromJavaScript extracts parameters from JavaScript code.
func (p *ParameterDiscovery) ExtractFromJavaScript(js string) []Parameter {
	params := make([]Parameter, 0)
	seen := make(map[string]bool)

	patterns := []*regexp.Regexp{
		// URL query parameters
		regexp.MustCompile(`[?&]([a-zA-Z0-9_]+)=`),

		// Object keys that look like parameters
		regexp.MustCompile(`["']?(params|data|query|body|payload)["']?\s*[=:]\s*\{([^}]+)\}`),

		// Direct parameter assignments
		regexp.MustCompile(`(?:params|data|query|body)\[["']([a-zA-Z0-9_]+)["']\]`),
		regexp.MustCompile(`(?:params|data|query|body)\.([a-zA-Z0-9_]+)\s*=`),

		// FormData append
		regexp.MustCompile(`formData\.append\s*\(\s*["']([^"']+)["']`),

		// URLSearchParams
		regexp.MustCompile(`searchParams\.(?:set|append)\s*\(\s*["']([^"']+)["']`),

		// Axios/fetch body
		regexp.MustCompile(`(?:data|body)\s*:\s*\{[^}]*["']([a-zA-Z0-9_]+)["']\s*:`),
	}

	for _, re := range patterns {
		matches := re.FindAllStringSubmatch(js, -1)
		for _, match := range matches {
			if len(match) > 1 {
				// For object patterns, extract individual keys
				if strings.Contains(match[0], "{") && len(match) > 2 {
					keyRe := regexp.MustCompile(`["']?([a-zA-Z0-9_]+)["']?\s*:`)
					keyMatches := keyRe.FindAllStringSubmatch(match[2], -1)
					for _, km := range keyMatches {
						if len(km) > 1 && !seen[km[1]] {
							seen[km[1]] = true
							params = append(params, Parameter{
								Name:   km[1],
								Type:   "body",
								Source: "javascript",
							})
						}
					}
				} else {
					paramName := match[1]
					if !seen[paramName] && isValidParamName(paramName) {
						seen[paramName] = true
						params = append(params, Parameter{
							Name:   paramName,
							Type:   "query",
							Source: "javascript",
						})
					}
				}
			}
		}
	}

	// API endpoint parameter extraction
	apiPatterns := []*regexp.Regexp{
		regexp.MustCompile(`/api/[^"'\s]*\{([a-zA-Z0-9_]+)\}`),
		regexp.MustCompile(`/api/[^"'\s]*:([a-zA-Z0-9_]+)`),
		regexp.MustCompile(`\$\{([a-zA-Z0-9_]+)\}`),
	}

	for _, re := range apiPatterns {
		matches := re.FindAllStringSubmatch(js, -1)
		for _, match := range matches {
			if len(match) > 1 && !seen[match[1]] {
				seen[match[1]] = true
				params = append(params, Parameter{
					Name:   match[1],
					Type:   "path",
					Source: "javascript-template",
				})
			}
		}
	}

	return params
}

// extractPathParams extracts potential path parameters from URL paths.
func (p *ParameterDiscovery) extractPathParams(path string, result *ParameterResult) {
	// Common patterns for path parameters
	parts := strings.Split(path, "/")

	for i, part := range parts {
		if part == "" {
			continue
		}

		// Check if it looks like a numeric ID
		if isNumericID(part) && i > 0 {
			// The previous part might indicate what this ID is for
			prevPart := ""
			if i > 0 {
				prevPart = parts[i-1]
			}

			paramKey := "path_" + prevPart + "_id"
			if !p.seen[paramKey] {
				p.seen[paramKey] = true
				result.PathParams = append(result.PathParams, Parameter{
					Name:    singularize(prevPart) + "_id",
					Value:   part,
					Type:    "path",
					Source:  "url-path",
					Context: path,
				})
			}
		}

		// Check for UUID-like patterns
		if isUUID(part) && i > 0 {
			prevPart := parts[i-1]
			paramKey := "path_" + prevPart + "_uuid"
			if !p.seen[paramKey] {
				p.seen[paramKey] = true
				result.PathParams = append(result.PathParams, Parameter{
					Name:    singularize(prevPart) + "_id",
					Value:   part,
					Type:    "path",
					Source:  "url-path-uuid",
					Context: path,
				})
			}
		}
	}
}

// CommonParameters returns a list of commonly used parameter names.
func (p *ParameterDiscovery) CommonParameters() []string {
	return []string{
		// Pagination
		"page", "limit", "offset", "size", "per_page", "pageSize",
		"start", "count", "skip", "take",

		// Sorting
		"sort", "order", "orderBy", "sortBy", "direction", "asc", "desc",

		// Filtering
		"filter", "search", "query", "q", "keyword", "keywords",
		"status", "type", "category", "tag", "tags",
		"from", "to", "start_date", "end_date", "date",

		// IDs
		"id", "ids", "user_id", "userId", "account_id", "accountId",
		"item_id", "itemId", "product_id", "productId",

		// Auth
		"token", "access_token", "refresh_token", "api_key", "apiKey",
		"auth", "authorization", "session", "session_id", "sessionId",

		// Actions
		"action", "cmd", "command", "op", "operation",
		"method", "mode", "format", "output",

		// Common fields
		"name", "email", "username", "password", "phone",
		"address", "city", "country", "zip", "postal_code",

		// Debug/Admin
		"debug", "test", "admin", "verbose", "trace",

		// File operations
		"file", "filename", "path", "url", "uri", "src",
		"upload", "download", "attachment",

		// Callbacks
		"callback", "redirect", "return_url", "returnUrl",
		"next", "back", "continue", "goto",

		// Misc
		"lang", "language", "locale", "currency",
		"version", "v", "ref", "source", "utm_source",
	}
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func isCommonDataAttr(name string) bool {
	common := []string{
		"reactroot", "reactid", "v", "vue", "ng", "ember",
		"testid", "test-id", "cy", "toggle", "target", "dismiss",
		"ride", "slide", "backdrop", "keyboard", "focus",
	}
	nameLower := strings.ToLower(name)
	for _, c := range common {
		if strings.Contains(nameLower, c) {
			return true
		}
	}
	return false
}

func isValidParamName(name string) bool {
	if len(name) < 2 || len(name) > 50 {
		return false
	}
	// Should start with letter
	if !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
		return false
	}
	// Should only contain alphanumeric and underscore
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func isNumericID(s string) bool {
	if len(s) == 0 || len(s) > 20 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isUUID(s string) bool {
	// Simple UUID pattern check (8-4-4-4-12)
	if len(s) != 36 {
		return false
	}
	uuidRe := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	return uuidRe.MatchString(s)
}

func singularize(word string) string {
	// Very simple singularization
	if strings.HasSuffix(word, "ies") {
		return strings.TrimSuffix(word, "ies") + "y"
	}
	if strings.HasSuffix(word, "es") {
		return strings.TrimSuffix(word, "es")
	}
	if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") {
		return strings.TrimSuffix(word, "s")
	}
	return word
}
