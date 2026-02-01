// Package parser provides HTML and JavaScript parsing for the crawler.
package parser

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// HTMLParser parses HTML documents to extract links and other elements.
type HTMLParser struct {
	baseURL *url.URL
}

// NewHTMLParser creates a new HTML parser.
func NewHTMLParser(baseURL string) (*HTMLParser, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &HTMLParser{baseURL: u}, nil
}

// ParseResult contains the result of parsing an HTML document.
type ParseResult struct {
	Links       []Link
	Forms       []FormInfo
	Scripts     []string
	Stylesheets []string
	Images      []string
	Iframes     []string
	Meta        map[string]string
	Comments    []string
}

// Link represents a parsed link.
type Link struct {
	URL      string
	Text     string
	Rel      string
	Target   string
	NoFollow bool
}

// FormInfo represents a parsed form.
type FormInfo struct {
	Action   string
	Method   string
	Enctype  string
	ID       string
	Name     string
	Class    string
	Inputs   []InputInfo
	Buttons  []ButtonInfo
}

// InputInfo represents a form input.
type InputInfo struct {
	Name        string
	Type        string
	Value       string
	ID          string
	Class       string
	Required    bool
	Disabled    bool
	Readonly    bool
	Placeholder string
	Pattern     string
	MinLength   int
	MaxLength   int
	Min         string
	Max         string
	Step        string
	Multiple    bool
	Accept      string
	Autocomplete string
}

// ButtonInfo represents a form button.
type ButtonInfo struct {
	Name     string
	Type     string
	Value    string
	Text     string
	FormAction string
}

// Parse parses an HTML document.
func (p *HTMLParser) Parse(html string) (*ParseResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	result := &ParseResult{
		Links:       make([]Link, 0),
		Forms:       make([]FormInfo, 0),
		Scripts:     make([]string, 0),
		Stylesheets: make([]string, 0),
		Images:      make([]string, 0),
		Iframes:     make([]string, 0),
		Meta:        make(map[string]string),
		Comments:    make([]string, 0),
	}

	// Extract links from href attributes
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		resolved := p.resolveURL(href)
		if resolved == "" {
			return
		}

		link := Link{
			URL:  resolved,
			Text: strings.TrimSpace(s.Text()),
		}

		if rel, exists := s.Attr("rel"); exists {
			link.Rel = rel
			link.NoFollow = strings.Contains(rel, "nofollow")
		}
		if target, exists := s.Attr("target"); exists {
			link.Target = target
		}

		result.Links = append(result.Links, link)
	})

	// Extract AngularJS ng-href links
	doc.Find("[ng-href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("ng-href")
		if !exists || href == "" {
			return
		}
		// ng-href may contain Angular expressions like {{url}}
		// Skip if it contains unresolved expressions
		if strings.Contains(href, "{{") {
			return
		}
		resolved := p.resolveURL(href)
		if resolved != "" {
			result.Links = append(result.Links, Link{
				URL:  resolved,
				Text: strings.TrimSpace(s.Text()),
			})
		}
	})

	// Extract Angular UI Router ui-sref links
	doc.Find("[ui-sref]").Each(func(i int, s *goquery.Selection) {
		sref, exists := s.Attr("ui-sref")
		if !exists || sref == "" {
			return
		}
		// ui-sref format: stateName or stateName({param: value})
		// Extract state name and convert to hash route
		stateName := sref
		if idx := strings.Index(sref, "("); idx != -1 {
			stateName = sref[:idx]
		}
		stateName = strings.TrimSpace(stateName)
		if stateName != "" {
			hashURL := p.baseURL.Scheme + "://" + p.baseURL.Host + "/#/" + stateName
			result.Links = append(result.Links, Link{
				URL:  hashURL,
				Text: strings.TrimSpace(s.Text()),
			})
		}
	})

	// Extract routerLink for Angular 2+ apps
	doc.Find("[routerLink]").Each(func(i int, s *goquery.Selection) {
		routerLink, exists := s.Attr("routerLink")
		if !exists || routerLink == "" {
			return
		}
		// routerLink can be a string like "/users" or an array
		routerLink = strings.Trim(routerLink, "[]\"'")
		if routerLink != "" && !strings.HasPrefix(routerLink, "/") {
			routerLink = "/" + routerLink
		}
		resolved := p.resolveURL(routerLink)
		if resolved != "" {
			result.Links = append(result.Links, Link{
				URL:  resolved,
				Text: strings.TrimSpace(s.Text()),
			})
		}
	})

	// Extract forms
	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		form := p.parseForm(s)
		result.Forms = append(result.Forms, form)
	})

	// Extract scripts
	doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			resolved := p.resolveURL(src)
			if resolved != "" {
				result.Scripts = append(result.Scripts, resolved)
			}
		}
	})

	// Extract stylesheets
	doc.Find("link[rel='stylesheet'], link[type='text/css']").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			resolved := p.resolveURL(href)
			if resolved != "" {
				result.Stylesheets = append(result.Stylesheets, resolved)
			}
		}
	})

	// Extract images
	doc.Find("img[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			resolved := p.resolveURL(src)
			if resolved != "" {
				result.Images = append(result.Images, resolved)
			}
		}
	})

	// Extract iframes
	doc.Find("iframe[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			resolved := p.resolveURL(src)
			if resolved != "" {
				result.Iframes = append(result.Iframes, resolved)
			}
		}
	})

	// Extract meta tags
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		property, _ := s.Attr("property")
		content, _ := s.Attr("content")

		key := name
		if key == "" {
			key = property
		}
		if key != "" && content != "" {
			result.Meta[key] = content
		}
	})

	return result, nil
}

// parseForm parses a form element.
func (p *HTMLParser) parseForm(s *goquery.Selection) FormInfo {
	form := FormInfo{
		Inputs:  make([]InputInfo, 0),
		Buttons: make([]ButtonInfo, 0),
	}

	if action, exists := s.Attr("action"); exists {
		form.Action = p.resolveURL(action)
	} else {
		form.Action = p.baseURL.String()
	}

	if method, exists := s.Attr("method"); exists {
		form.Method = strings.ToUpper(method)
	} else {
		form.Method = "GET"
	}

	if enctype, exists := s.Attr("enctype"); exists {
		form.Enctype = enctype
	} else {
		form.Enctype = "application/x-www-form-urlencoded"
	}

	form.ID, _ = s.Attr("id")
	form.Name, _ = s.Attr("name")
	form.Class, _ = s.Attr("class")

	// Parse inputs
	s.Find("input, textarea, select").Each(func(i int, input *goquery.Selection) {
		info := p.parseInput(input)
		form.Inputs = append(form.Inputs, info)
	})

	// Parse buttons
	s.Find("button, input[type='submit'], input[type='button']").Each(func(i int, btn *goquery.Selection) {
		button := ButtonInfo{}
		button.Name, _ = btn.Attr("name")
		button.Value, _ = btn.Attr("value")
		button.FormAction, _ = btn.Attr("formaction")

		if btn.Is("button") {
			button.Text = strings.TrimSpace(btn.Text())
			button.Type, _ = btn.Attr("type")
			if button.Type == "" {
				button.Type = "submit"
			}
		} else {
			button.Type, _ = btn.Attr("type")
		}

		form.Buttons = append(form.Buttons, button)
	})

	return form
}

// parseInput parses an input element.
func (p *HTMLParser) parseInput(s *goquery.Selection) InputInfo {
	info := InputInfo{}

	info.Name, _ = s.Attr("name")
	info.ID, _ = s.Attr("id")
	info.Class, _ = s.Attr("class")
	info.Value, _ = s.Attr("value")
	info.Placeholder, _ = s.Attr("placeholder")
	info.Pattern, _ = s.Attr("pattern")
	info.Min, _ = s.Attr("min")
	info.Max, _ = s.Attr("max")
	info.Step, _ = s.Attr("step")
	info.Accept, _ = s.Attr("accept")
	info.Autocomplete, _ = s.Attr("autocomplete")

	if s.Is("textarea") {
		info.Type = "textarea"
		info.Value = strings.TrimSpace(s.Text())
	} else if s.Is("select") {
		info.Type = "select"
		// Get first option value
		s.Find("option").First().Each(func(i int, opt *goquery.Selection) {
			info.Value, _ = opt.Attr("value")
		})
	} else {
		info.Type, _ = s.Attr("type")
		if info.Type == "" {
			info.Type = "text"
		}
	}

	_, info.Required = s.Attr("required")
	_, info.Disabled = s.Attr("disabled")
	_, info.Readonly = s.Attr("readonly")
	_, info.Multiple = s.Attr("multiple")

	return info
}

// resolveURL resolves a relative URL against the base URL.
func (p *HTMLParser) resolveURL(href string) string {
	if href == "" {
		return ""
	}

	// Skip javascript:, mailto:, tel:, data: URLs
	lower := strings.ToLower(href)
	if strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "data:") {
		return ""
	}

	// Handle hash-based SPA routing (e.g., #/path, #!/path)
	if strings.HasPrefix(href, "#") {
		// Skip simple anchors like #top, #section1
		// But keep SPA routes like #/, #/users, #!/login
		if len(href) > 1 && (href[1] == '/' || href[1] == '!') {
			// This is likely an SPA route - append to base URL
			baseStr := p.baseURL.Scheme + "://" + p.baseURL.Host
			return baseStr + "/" + href
		}
		return ""
	}

	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}

	resolved := p.baseURL.ResolveReference(ref)
	return resolved.String()
}

// ExtractURLsFromText extracts URLs from plain text.
func ExtractURLsFromText(text string) []string {
	urls := make([]string, 0)

	// Simple URL pattern matching
	patterns := []string{
		"https://",
		"http://",
	}

	for _, pattern := range patterns {
		idx := 0
		for {
			pos := strings.Index(text[idx:], pattern)
			if pos == -1 {
				break
			}

			start := idx + pos
			end := start
			for end < len(text) && !isURLTerminator(text[end]) {
				end++
			}

			urlStr := text[start:end]
			if _, err := url.Parse(urlStr); err == nil {
				urls = append(urls, urlStr)
			}

			idx = end
		}
	}

	return urls
}

func isURLTerminator(c byte) bool {
	terminators := " \t\n\r\"'<>()[]{}"
	for _, t := range terminators {
		if c == byte(t) {
			return true
		}
	}
	return false
}
