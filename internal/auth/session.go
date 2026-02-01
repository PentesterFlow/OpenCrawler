package auth

import (
	"context"
	"net/http"
	"sync"

	"github.com/PentesterFlow/OpenCrawler/internal/browser"
)

// SessionAuth provides session/cookie-based authentication.
type SessionAuth struct {
	mu            sync.RWMutex
	cookies       []*http.Cookie
	authenticated bool
}

// NewSessionAuth creates a new session authentication provider.
func NewSessionAuth(cookies []*http.Cookie) *SessionAuth {
	return &SessionAuth{
		cookies:       cookies,
		authenticated: len(cookies) > 0,
	}
}

// Authenticate sets up session authentication.
func (s *SessionAuth) Authenticate(ctx context.Context, pool *browser.Pool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.cookies) > 0 {
		s.authenticated = true
	}

	return nil
}

// GetHeaders returns empty headers for session auth.
func (s *SessionAuth) GetHeaders() map[string]string {
	return nil
}

// GetCookies returns the session cookies.
func (s *SessionAuth) GetCookies() []*http.Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*http.Cookie, len(s.cookies))
	copy(result, s.cookies)
	return result
}

// RefreshIfNeeded checks if cookies need refresh.
func (s *SessionAuth) RefreshIfNeeded(ctx context.Context) error {
	// Session cookies typically don't need refresh
	// Could implement session validation here
	return nil
}

// IsAuthenticated returns true if cookies are set.
func (s *SessionAuth) IsAuthenticated() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.authenticated
}

// Type returns the authentication type.
func (s *SessionAuth) Type() AuthType {
	return AuthTypeSession
}

// SetCookies updates the session cookies.
func (s *SessionAuth) SetCookies(cookies []*http.Cookie) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cookies = cookies
	s.authenticated = len(cookies) > 0
}

// AddCookie adds a cookie to the session.
func (s *SessionAuth) AddCookie(cookie *http.Cookie) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Replace if exists, otherwise add
	for i, c := range s.cookies {
		if c.Name == cookie.Name && c.Domain == cookie.Domain {
			s.cookies[i] = cookie
			return
		}
	}

	s.cookies = append(s.cookies, cookie)
	s.authenticated = true
}

// GetCookie returns a specific cookie by name.
func (s *SessionAuth) GetCookie(name string) *http.Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ClearCookies removes all cookies.
func (s *SessionAuth) ClearCookies() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cookies = nil
	s.authenticated = false
}
