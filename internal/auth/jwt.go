package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/browser"
)

// JWTAuth provides JWT-based authentication.
type JWTAuth struct {
	mu            sync.RWMutex
	token         string
	refreshToken  string
	expiry        time.Time
	refreshURL    string
	authenticated bool
}

// NewJWTAuth creates a new JWT authentication provider.
func NewJWTAuth(token string) *JWTAuth {
	auth := &JWTAuth{
		token:         token,
		authenticated: token != "",
	}

	// Try to parse expiry from token
	if token != "" {
		if exp, err := auth.parseExpiry(token); err == nil {
			auth.expiry = exp
		}
	}

	return auth
}

// NewJWTAuthWithRefresh creates a JWT auth with refresh capability.
func NewJWTAuthWithRefresh(token, refreshToken, refreshURL string) *JWTAuth {
	auth := NewJWTAuth(token)
	auth.refreshToken = refreshToken
	auth.refreshURL = refreshURL
	return auth
}

// Authenticate sets up JWT authentication.
func (j *JWTAuth) Authenticate(ctx context.Context, pool *browser.Pool) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.token != "" {
		j.authenticated = true
	}

	return nil
}

// GetHeaders returns the Authorization header.
func (j *JWTAuth) GetHeaders() map[string]string {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.token == "" {
		return nil
	}

	return map[string]string{
		"Authorization": "Bearer " + j.token,
	}
}

// GetCookies returns no cookies for JWT auth.
func (j *JWTAuth) GetCookies() []*http.Cookie {
	return nil
}

// RefreshIfNeeded refreshes the token if expired or expiring soon.
func (j *JWTAuth) RefreshIfNeeded(ctx context.Context) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	// Check if refresh is needed (within 5 minutes of expiry)
	if !j.expiry.IsZero() && time.Until(j.expiry) > 5*time.Minute {
		return nil
	}

	// Can't refresh without refresh token/URL
	if j.refreshToken == "" || j.refreshURL == "" {
		return nil
	}

	// Perform refresh
	return j.doRefresh(ctx)
}

// doRefresh performs the token refresh.
func (j *JWTAuth) doRefresh(ctx context.Context) error {
	// Create refresh request
	req, err := http.NewRequestWithContext(ctx, "POST", j.refreshURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+j.refreshToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh failed with status %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	j.token = result.AccessToken
	if result.RefreshToken != "" {
		j.refreshToken = result.RefreshToken
	}

	// Update expiry
	if exp, err := j.parseExpiry(j.token); err == nil {
		j.expiry = exp
	}

	return nil
}

// parseExpiry extracts the expiration time from a JWT.
func (j *JWTAuth) parseExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid JWT format")
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Try with standard encoding
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}, err
		}
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}

	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, err
	}

	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("no exp claim")
	}

	return time.Unix(claims.Exp, 0), nil
}

// IsAuthenticated returns true if we have a valid token.
func (j *JWTAuth) IsAuthenticated() bool {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if !j.authenticated || j.token == "" {
		return false
	}

	// Check expiry
	if !j.expiry.IsZero() && time.Now().After(j.expiry) {
		return false
	}

	return true
}

// Type returns the authentication type.
func (j *JWTAuth) Type() AuthType {
	return AuthTypeJWT
}

// SetToken updates the JWT token.
func (j *JWTAuth) SetToken(token string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.token = token
	j.authenticated = token != ""

	if exp, err := j.parseExpiry(token); err == nil {
		j.expiry = exp
	}
}

// GetToken returns the current token.
func (j *JWTAuth) GetToken() string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.token
}

// GetExpiry returns the token expiry time.
func (j *JWTAuth) GetExpiry() time.Time {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.expiry
}

// TimeUntilExpiry returns the time until the token expires.
func (j *JWTAuth) TimeUntilExpiry() time.Duration {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.expiry.IsZero() {
		return 0
	}

	return time.Until(j.expiry)
}
