package enhanced

import (
	"net/http"
	"regexp"
	"strings"
	"time"
)

// TechFingerprinter detects technologies used by a web application.
type TechFingerprinter struct {
	client    *http.Client
	userAgent string
}

// Technology represents a detected technology.
type Technology struct {
	Name       string
	Category   string
	Version    string
	Confidence int // 0-100
	Evidence   string
}

// TechResult contains all detected technologies.
type TechResult struct {
	Technologies []Technology
	Headers      map[string]string
	Cookies      []string
	MetaTags     map[string]string
}

// NewTechFingerprinter creates a new technology fingerprinter.
func NewTechFingerprinter(userAgent string) *TechFingerprinter {
	return &TechFingerprinter{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		userAgent: userAgent,
	}
}

// Fingerprint detects technologies from headers and HTML content.
func (f *TechFingerprinter) Fingerprint(headers http.Header, htmlContent string, cookies []*http.Cookie) *TechResult {
	result := &TechResult{
		Technologies: make([]Technology, 0),
		Headers:      make(map[string]string),
		Cookies:      make([]string, 0),
		MetaTags:     make(map[string]string),
	}

	// Store headers for reference
	for key, values := range headers {
		result.Headers[key] = strings.Join(values, ", ")
	}

	// Store cookie names
	for _, cookie := range cookies {
		result.Cookies = append(result.Cookies, cookie.Name)
	}

	// Detect from headers
	f.detectFromHeaders(headers, result)

	// Detect from cookies
	f.detectFromCookies(cookies, result)

	// Detect from HTML content
	f.detectFromHTML(htmlContent, result)

	return result
}

// detectFromHeaders detects technologies from HTTP headers.
func (f *TechFingerprinter) detectFromHeaders(headers http.Header, result *TechResult) {
	// Server header
	if server := headers.Get("Server"); server != "" {
		f.parseServerHeader(server, result)
	}

	// X-Powered-By
	if poweredBy := headers.Get("X-Powered-By"); poweredBy != "" {
		f.parsePoweredByHeader(poweredBy, result)
	}

	// X-AspNet-Version
	if aspnet := headers.Get("X-AspNet-Version"); aspnet != "" {
		result.Technologies = append(result.Technologies, Technology{
			Name:       "ASP.NET",
			Category:   "framework",
			Version:    aspnet,
			Confidence: 100,
			Evidence:   "X-AspNet-Version header",
		})
	}

	// X-AspNetMvc-Version
	if mvc := headers.Get("X-AspNetMvc-Version"); mvc != "" {
		result.Technologies = append(result.Technologies, Technology{
			Name:       "ASP.NET MVC",
			Category:   "framework",
			Version:    mvc,
			Confidence: 100,
			Evidence:   "X-AspNetMvc-Version header",
		})
	}

	// X-Generator
	if gen := headers.Get("X-Generator"); gen != "" {
		result.Technologies = append(result.Technologies, Technology{
			Name:       gen,
			Category:   "cms",
			Version:    "",
			Confidence: 90,
			Evidence:   "X-Generator header",
		})
	}

	// X-Drupal-Cache / X-Drupal-Dynamic-Cache
	if headers.Get("X-Drupal-Cache") != "" || headers.Get("X-Drupal-Dynamic-Cache") != "" {
		result.Technologies = append(result.Technologies, Technology{
			Name:       "Drupal",
			Category:   "cms",
			Confidence: 100,
			Evidence:   "X-Drupal-Cache header",
		})
	}

	// Via header (CDN/Proxy)
	if via := headers.Get("Via"); via != "" {
		if strings.Contains(strings.ToLower(via), "cloudflare") {
			result.Technologies = append(result.Technologies, Technology{
				Name:       "Cloudflare",
				Category:   "cdn",
				Confidence: 100,
				Evidence:   "Via header",
			})
		}
		if strings.Contains(strings.ToLower(via), "varnish") {
			result.Technologies = append(result.Technologies, Technology{
				Name:       "Varnish",
				Category:   "cache",
				Confidence: 100,
				Evidence:   "Via header",
			})
		}
	}

	// CF-Ray (Cloudflare)
	if headers.Get("CF-Ray") != "" {
		result.Technologies = append(result.Technologies, Technology{
			Name:       "Cloudflare",
			Category:   "cdn",
			Confidence: 100,
			Evidence:   "CF-Ray header",
		})
	}

	// X-Cache (various CDNs)
	if cache := headers.Get("X-Cache"); cache != "" {
		if strings.Contains(cache, "cloudfront") {
			result.Technologies = append(result.Technologies, Technology{
				Name:       "Amazon CloudFront",
				Category:   "cdn",
				Confidence: 100,
				Evidence:   "X-Cache header",
			})
		}
	}

	// X-Vercel-Id
	if headers.Get("X-Vercel-Id") != "" {
		result.Technologies = append(result.Technologies, Technology{
			Name:       "Vercel",
			Category:   "hosting",
			Confidence: 100,
			Evidence:   "X-Vercel-Id header",
		})
	}

	// X-Amz-* (AWS)
	for key := range headers {
		if strings.HasPrefix(strings.ToLower(key), "x-amz-") {
			result.Technologies = append(result.Technologies, Technology{
				Name:       "Amazon Web Services",
				Category:   "hosting",
				Confidence: 90,
				Evidence:   key + " header",
			})
			break
		}
	}
}

// parseServerHeader parses the Server header.
func (f *TechFingerprinter) parseServerHeader(server string, result *TechResult) {
	serverLower := strings.ToLower(server)

	patterns := []struct {
		pattern  string
		name     string
		category string
	}{
		{`nginx/?(\d+\.[\d.]+)?`, "nginx", "web-server"},
		{`apache/?(\d+\.[\d.]+)?`, "Apache", "web-server"},
		{`iis/?(\d+\.[\d.]+)?`, "Microsoft IIS", "web-server"},
		{`lighttpd/?(\d+\.[\d.]+)?`, "lighttpd", "web-server"},
		{`caddy/?(\d+\.[\d.]+)?`, "Caddy", "web-server"},
		{`openresty/?(\d+\.[\d.]+)?`, "OpenResty", "web-server"},
		{`litespeed`, "LiteSpeed", "web-server"},
		{`cloudflare`, "Cloudflare", "cdn"},
		{`gunicorn/?(\d+\.[\d.]+)?`, "Gunicorn", "web-server"},
		{`uvicorn`, "Uvicorn", "web-server"},
		{`daphne`, "Daphne", "web-server"},
		{`werkzeug/?(\d+\.[\d.]+)?`, "Werkzeug", "framework"},
		{`tornado/?(\d+\.[\d.]+)?`, "Tornado", "framework"},
		{`jetty/?(\d+\.[\d.]+)?`, "Jetty", "web-server"},
		{`tomcat/?(\d+\.[\d.]+)?`, "Apache Tomcat", "web-server"},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(`(?i)` + p.pattern)
		if matches := re.FindStringSubmatch(serverLower); matches != nil {
			version := ""
			if len(matches) > 1 {
				version = matches[1]
			}
			result.Technologies = append(result.Technologies, Technology{
				Name:       p.name,
				Category:   p.category,
				Version:    version,
				Confidence: 100,
				Evidence:   "Server header: " + server,
			})
		}
	}
}

// parsePoweredByHeader parses the X-Powered-By header.
func (f *TechFingerprinter) parsePoweredByHeader(poweredBy string, result *TechResult) {
	poweredByLower := strings.ToLower(poweredBy)

	patterns := []struct {
		pattern  string
		name     string
		category string
	}{
		{`php/?(\d+\.[\d.]+)?`, "PHP", "language"},
		{`asp\.net`, "ASP.NET", "framework"},
		{`express`, "Express", "framework"},
		{`next\.js/?(\d+\.[\d.]+)?`, "Next.js", "framework"},
		{`nuxt\.js/?(\d+\.[\d.]+)?`, "Nuxt.js", "framework"},
		{`flask/?(\d+\.[\d.]+)?`, "Flask", "framework"},
		{`django/?(\d+\.[\d.]+)?`, "Django", "framework"},
		{`rails/?(\d+\.[\d.]+)?`, "Ruby on Rails", "framework"},
		{`laravel/?(\d+\.[\d.]+)?`, "Laravel", "framework"},
		{`symfony/?(\d+\.[\d.]+)?`, "Symfony", "framework"},
		{`spring/?(\d+\.[\d.]+)?`, "Spring", "framework"},
		{`kestrel`, "Kestrel", "web-server"},
		{`plone/?(\d+\.[\d.]+)?`, "Plone", "cms"},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(`(?i)` + p.pattern)
		if matches := re.FindStringSubmatch(poweredByLower); matches != nil {
			version := ""
			if len(matches) > 1 {
				version = matches[1]
			}
			result.Technologies = append(result.Technologies, Technology{
				Name:       p.name,
				Category:   p.category,
				Version:    version,
				Confidence: 100,
				Evidence:   "X-Powered-By header: " + poweredBy,
			})
		}
	}
}

// detectFromCookies detects technologies from cookie names.
func (f *TechFingerprinter) detectFromCookies(cookies []*http.Cookie, result *TechResult) {
	cookiePatterns := map[string]Technology{
		"PHPSESSID":           {Name: "PHP", Category: "language", Confidence: 100, Evidence: "PHPSESSID cookie"},
		"ASP.NET_SessionId":   {Name: "ASP.NET", Category: "framework", Confidence: 100, Evidence: "ASP.NET_SessionId cookie"},
		"JSESSIONID":          {Name: "Java", Category: "language", Confidence: 100, Evidence: "JSESSIONID cookie"},
		"rack.session":        {Name: "Ruby", Category: "language", Confidence: 100, Evidence: "rack.session cookie"},
		"_session_id":         {Name: "Ruby on Rails", Category: "framework", Confidence: 80, Evidence: "_session_id cookie"},
		"csrftoken":           {Name: "Django", Category: "framework", Confidence: 80, Evidence: "csrftoken cookie"},
		"django_session":      {Name: "Django", Category: "framework", Confidence: 100, Evidence: "django_session cookie"},
		"laravel_session":     {Name: "Laravel", Category: "framework", Confidence: 100, Evidence: "laravel_session cookie"},
		"XSRF-TOKEN":          {Name: "Laravel/Angular", Category: "framework", Confidence: 70, Evidence: "XSRF-TOKEN cookie"},
		"connect.sid":         {Name: "Express", Category: "framework", Confidence: 90, Evidence: "connect.sid cookie"},
		"wp-settings":         {Name: "WordPress", Category: "cms", Confidence: 100, Evidence: "wp-settings cookie"},
		"wordpress_logged_in": {Name: "WordPress", Category: "cms", Confidence: 100, Evidence: "wordpress_logged_in cookie"},
		"Drupal.visitor":      {Name: "Drupal", Category: "cms", Confidence: 100, Evidence: "Drupal.visitor cookie"},
		"joomla_user_state":   {Name: "Joomla", Category: "cms", Confidence: 100, Evidence: "joomla_user_state cookie"},
		"__cf_bm":             {Name: "Cloudflare", Category: "cdn", Confidence: 100, Evidence: "__cf_bm cookie"},
		"_ga":                 {Name: "Google Analytics", Category: "analytics", Confidence: 100, Evidence: "_ga cookie"},
		"_gid":                {Name: "Google Analytics", Category: "analytics", Confidence: 100, Evidence: "_gid cookie"},
		"_fbp":                {Name: "Facebook Pixel", Category: "analytics", Confidence: 100, Evidence: "_fbp cookie"},
	}

	for _, cookie := range cookies {
		if tech, ok := cookiePatterns[cookie.Name]; ok {
			result.Technologies = append(result.Technologies, tech)
		}
	}
}

// detectFromHTML detects technologies from HTML content.
func (f *TechFingerprinter) detectFromHTML(html string, result *TechResult) {
	htmlLower := strings.ToLower(html)

	// JavaScript frameworks detection
	jsFrameworks := []struct {
		pattern    string
		name       string
		category   string
		confidence int
	}{
		// React
		{`data-reactroot`, "React", "javascript-framework", 100},
		{`__NEXT_DATA__`, "Next.js", "javascript-framework", 100},
		{`_react`, "React", "javascript-framework", 90},

		// Vue
		{`data-v-`, "Vue.js", "javascript-framework", 100},
		{`__NUXT__`, "Nuxt.js", "javascript-framework", 100},
		{`vue\.js`, "Vue.js", "javascript-framework", 80},
		{`v-cloak`, "Vue.js", "javascript-framework", 100},

		// Angular
		{`ng-version`, "Angular", "javascript-framework", 100},
		{`ng-app`, "AngularJS", "javascript-framework", 100},
		{`ng-controller`, "AngularJS", "javascript-framework", 100},
		{`_ng`, "Angular", "javascript-framework", 80},

		// Ember
		{`data-ember`, "Ember.js", "javascript-framework", 100},
		{`ember-view`, "Ember.js", "javascript-framework", 100},

		// Svelte
		{`svelte`, "Svelte", "javascript-framework", 80},

		// jQuery
		{`jquery`, "jQuery", "javascript-library", 80},

		// Bootstrap
		{`bootstrap\.min\.css`, "Bootstrap", "css-framework", 100},
		{`bootstrap\.css`, "Bootstrap", "css-framework", 100},

		// Tailwind
		{`tailwind`, "Tailwind CSS", "css-framework", 80},

		// CMS detection
		{`wp-content`, "WordPress", "cms", 100},
		{`wp-includes`, "WordPress", "cms", 100},
		{`/drupal`, "Drupal", "cms", 90},
		{`joomla`, "Joomla", "cms", 80},
		{`shopify`, "Shopify", "ecommerce", 90},
		{`magento`, "Magento", "ecommerce", 90},
		{`woocommerce`, "WooCommerce", "ecommerce", 100},

		// Analytics
		{`google-analytics\.com`, "Google Analytics", "analytics", 100},
		{`googletagmanager\.com`, "Google Tag Manager", "analytics", 100},
		{`facebook\.net`, "Facebook SDK", "analytics", 100},
		{`hotjar\.com`, "Hotjar", "analytics", 100},
		{`mixpanel\.com`, "Mixpanel", "analytics", 100},

		// Chat/Support
		{`intercom`, "Intercom", "support", 90},
		{`crisp\.chat`, "Crisp", "support", 100},
		{`zendesk`, "Zendesk", "support", 90},
		{`freshdesk`, "Freshdesk", "support", 90},

		// Other
		{`recaptcha`, "reCAPTCHA", "security", 100},
		{`hcaptcha`, "hCaptcha", "security", 100},
		{`cloudflare`, "Cloudflare", "cdn", 80},
		{`akamai`, "Akamai", "cdn", 80},
		{`stripe\.com`, "Stripe", "payment", 100},
		{`paypal\.com`, "PayPal", "payment", 100},
	}

	seen := make(map[string]bool)
	for _, jf := range jsFrameworks {
		re := regexp.MustCompile(`(?i)` + jf.pattern)
		if re.MatchString(htmlLower) && !seen[jf.name] {
			seen[jf.name] = true
			result.Technologies = append(result.Technologies, Technology{
				Name:       jf.name,
				Category:   jf.category,
				Confidence: jf.confidence,
				Evidence:   "HTML pattern: " + jf.pattern,
			})
		}
	}

	// Extract meta generator
	generatorRe := regexp.MustCompile(`<meta[^>]+name=["']generator["'][^>]+content=["']([^"']+)["']`)
	if matches := generatorRe.FindStringSubmatch(html); matches != nil {
		result.MetaTags["generator"] = matches[1]
		result.Technologies = append(result.Technologies, Technology{
			Name:       matches[1],
			Category:   "cms",
			Confidence: 100,
			Evidence:   "Meta generator tag",
		})
	}

	// Alternate generator pattern
	generatorRe2 := regexp.MustCompile(`<meta[^>]+content=["']([^"']+)["'][^>]+name=["']generator["']`)
	if matches := generatorRe2.FindStringSubmatch(html); matches != nil {
		if result.MetaTags["generator"] == "" {
			result.MetaTags["generator"] = matches[1]
			result.Technologies = append(result.Technologies, Technology{
				Name:       matches[1],
				Category:   "cms",
				Confidence: 100,
				Evidence:   "Meta generator tag",
			})
		}
	}
}

// GetSecurityRelevantTech returns technologies that are security-relevant.
func (f *TechFingerprinter) GetSecurityRelevantTech(result *TechResult) []Technology {
	relevant := make([]Technology, 0)

	for _, tech := range result.Technologies {
		// Web servers with known vulnerabilities
		if tech.Category == "web-server" && tech.Version != "" {
			relevant = append(relevant, tech)
		}

		// CMS systems
		if tech.Category == "cms" {
			relevant = append(relevant, tech)
		}

		// Languages/frameworks with version info
		if (tech.Category == "language" || tech.Category == "framework") && tech.Version != "" {
			relevant = append(relevant, tech)
		}

		// eCommerce platforms
		if tech.Category == "ecommerce" {
			relevant = append(relevant, tech)
		}
	}

	return relevant
}
