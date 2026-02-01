package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"

	"github.com/PentesterFlow/OpenCrawler/internal/browser"
)

// FormLoginAuth provides form-based login authentication.
type FormLoginAuth struct {
	mu            sync.RWMutex
	loginURL      string
	username      string
	password      string
	usernameField string
	passwordField string
	submitButton  string
	extraFields   map[string]string
	cookies       []*http.Cookie
	authenticated bool
	lastLogin     time.Time
	sessionLife   time.Duration
}

// NewFormLoginAuth creates a new form login authentication provider.
func NewFormLoginAuth(creds Credentials) *FormLoginAuth {
	auth := &FormLoginAuth{
		loginURL:      creds.LoginURL,
		username:      creds.Username,
		password:      creds.Password,
		usernameField: "username",
		passwordField: "password",
		extraFields:   creds.FormFields,
		sessionLife:   30 * time.Minute,
	}

	// Override field names if provided
	if creds.FormFields != nil {
		if f, ok := creds.FormFields["username_field"]; ok && f != "" {
			auth.usernameField = f
		}
		if f, ok := creds.FormFields["password_field"]; ok && f != "" {
			auth.passwordField = f
		}
		if f, ok := creds.FormFields["submit_button"]; ok && f != "" {
			auth.submitButton = f
		}
	}

	return auth
}

// Authenticate performs form-based login.
func (f *FormLoginAuth) Authenticate(ctx context.Context, pool *browser.Pool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.loginURL == "" {
		return fmt.Errorf("login URL is required")
	}

	// Get a browser instance
	b, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire browser: %w", err)
	}
	defer pool.Release(b)

	// Perform login using rod directly
	return f.performLogin(ctx)
}

// performLogin uses the browser to perform the login.
func (f *FormLoginAuth) performLogin(ctx context.Context) error {
	// Create a temporary browser for login
	browser := rod.New()
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("failed to connect browser: %w", err)
	}
	defer browser.Close()

	page, err := browser.Page(proto.TargetCreateTarget{URL: f.loginURL})
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Wait for page to load
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("failed to load login page: %w", err)
	}

	// Find and fill username field
	usernameSelectors := []string{
		fmt.Sprintf("input[name='%s']", f.usernameField),
		"input[type='email']",
		"input[type='text'][name*='user']",
		"input[type='text'][name*='email']",
		"input#username",
		"input#email",
	}

	var usernameElement *rod.Element
	for _, selector := range usernameSelectors {
		el, err := page.Element(selector)
		if err == nil && el != nil {
			usernameElement = el
			break
		}
	}

	if usernameElement == nil {
		return fmt.Errorf("could not find username field")
	}

	if err := usernameElement.SelectAllText(); err == nil {
		_ = usernameElement.Input(f.username)
	}

	// Find and fill password field
	passwordSelectors := []string{
		fmt.Sprintf("input[name='%s']", f.passwordField),
		"input[type='password']",
		"input#password",
	}

	var passwordElement *rod.Element
	for _, selector := range passwordSelectors {
		el, err := page.Element(selector)
		if err == nil && el != nil {
			passwordElement = el
			break
		}
	}

	if passwordElement == nil {
		return fmt.Errorf("could not find password field")
	}

	if err := passwordElement.SelectAllText(); err == nil {
		_ = passwordElement.Input(f.password)
	}

	// Find and click submit button
	submitSelectors := []string{
		"button[type='submit']",
		"input[type='submit']",
	}

	if f.submitButton != "" {
		submitSelectors = append([]string{f.submitButton}, submitSelectors...)
	}

	var submitElement *rod.Element
	for _, selector := range submitSelectors {
		el, err := page.Element(selector)
		if err == nil && el != nil {
			submitElement = el
			break
		}
	}

	if submitElement != nil {
		_ = submitElement.Click(proto.InputMouseButtonLeft, 1)
	} else {
		// Try pressing Enter on the password field
		_ = passwordElement.Type(input.Enter)
	}

	// Wait for navigation
	_ = page.WaitLoad()
	time.Sleep(2 * time.Second)

	// Extract cookies
	rodCookies, err := page.Cookies(nil)
	if err != nil {
		return fmt.Errorf("failed to get cookies: %w", err)
	}

	f.cookies = make([]*http.Cookie, 0, len(rodCookies))
	for _, c := range rodCookies {
		f.cookies = append(f.cookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}

	// Check if login was successful
	if !f.isLoginSuccessful(page) {
		return fmt.Errorf("login appears to have failed")
	}

	f.authenticated = true
	f.lastLogin = time.Now()

	return nil
}

// isLoginSuccessful checks if login was successful.
func (f *FormLoginAuth) isLoginSuccessful(page *rod.Page) bool {
	// Check if we're still on the login page
	info, err := page.Info()
	if err != nil {
		return len(f.cookies) > 0
	}

	currentURL := info.URL
	if strings.Contains(currentURL, "login") || strings.Contains(currentURL, "signin") {
		// Check for error messages
		errorSelectors := []string{
			".error",
			".alert-danger",
			".alert-error",
			"[class*='error']",
			"[class*='invalid']",
		}

		for _, selector := range errorSelectors {
			el, err := page.Element(selector)
			if err == nil && el != nil {
				text, _ := el.Text()
				if text != "" {
					return false
				}
			}
		}
	}

	// Check if we have session cookies
	return len(f.cookies) > 0
}

// GetHeaders returns no headers for form login.
func (f *FormLoginAuth) GetHeaders() map[string]string {
	return nil
}

// GetCookies returns the session cookies.
func (f *FormLoginAuth) GetCookies() []*http.Cookie {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]*http.Cookie, len(f.cookies))
	copy(result, f.cookies)
	return result
}

// RefreshIfNeeded re-authenticates if the session may have expired.
func (f *FormLoginAuth) RefreshIfNeeded(ctx context.Context) error {
	f.mu.RLock()
	needsRefresh := time.Since(f.lastLogin) > f.sessionLife
	f.mu.RUnlock()

	if !needsRefresh {
		return nil
	}

	// Need to re-authenticate - but we need a browser pool
	return nil
}

// IsAuthenticated returns true if we have session cookies.
func (f *FormLoginAuth) IsAuthenticated() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.authenticated && len(f.cookies) > 0
}

// Type returns the authentication type.
func (f *FormLoginAuth) Type() AuthType {
	return AuthTypeFormLogin
}

// SetSessionLifetime sets how long a session is considered valid.
func (f *FormLoginAuth) SetSessionLifetime(duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessionLife = duration
}

// APIKeyAuth provides API key authentication.
type APIKeyAuth struct {
	mu      sync.RWMutex
	headers map[string]string
}

// NewAPIKeyAuth creates a new API key authentication provider.
func NewAPIKeyAuth(headers map[string]string) *APIKeyAuth {
	return &APIKeyAuth{
		headers: headers,
	}
}

// Authenticate is a no-op for API key auth.
func (a *APIKeyAuth) Authenticate(ctx context.Context, pool *browser.Pool) error {
	return nil
}

// GetHeaders returns the API key headers.
func (a *APIKeyAuth) GetHeaders() map[string]string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range a.headers {
		result[k] = v
	}
	return result
}

// GetCookies returns no cookies for API key auth.
func (a *APIKeyAuth) GetCookies() []*http.Cookie {
	return nil
}

// RefreshIfNeeded is a no-op for API key auth.
func (a *APIKeyAuth) RefreshIfNeeded(ctx context.Context) error {
	return nil
}

// IsAuthenticated returns true if headers are set.
func (a *APIKeyAuth) IsAuthenticated() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.headers) > 0
}

// Type returns the authentication type.
func (a *APIKeyAuth) Type() AuthType {
	return AuthTypeAPIKey
}

// BasicAuth provides HTTP Basic authentication.
type BasicAuth struct {
	mu       sync.RWMutex
	username string
	password string
}

// NewBasicAuth creates a new Basic authentication provider.
func NewBasicAuth(username, password string) *BasicAuth {
	return &BasicAuth{
		username: username,
		password: password,
	}
}

// Authenticate is a no-op for Basic auth.
func (b *BasicAuth) Authenticate(ctx context.Context, pool *browser.Pool) error {
	return nil
}

// GetHeaders returns the Authorization header.
func (b *BasicAuth) GetHeaders() map[string]string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.username == "" && b.password == "" {
		return nil
	}

	// Base64 encode credentials
	creds := base64.StdEncoding.EncodeToString([]byte(b.username + ":" + b.password))

	return map[string]string{
		"Authorization": "Basic " + creds,
	}
}

// GetCookies returns no cookies for Basic auth.
func (b *BasicAuth) GetCookies() []*http.Cookie {
	return nil
}

// RefreshIfNeeded is a no-op for Basic auth.
func (b *BasicAuth) RefreshIfNeeded(ctx context.Context) error {
	return nil
}

// IsAuthenticated returns true if credentials are set.
func (b *BasicAuth) IsAuthenticated() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.username != "" || b.password != ""
}

// Type returns the authentication type.
func (b *BasicAuth) Type() AuthType {
	return AuthTypeBasic
}
