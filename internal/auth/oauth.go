package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/browser"
)

// OAuthAuth provides OAuth 2.0 authentication.
type OAuthAuth struct {
	mu            sync.RWMutex
	config        *OAuthConfig
	accessToken   string
	refreshToken  string
	tokenType     string
	expiry        time.Time
	authenticated bool
}

// NewOAuthAuth creates a new OAuth authentication provider.
func NewOAuthAuth(config *OAuthConfig) *OAuthAuth {
	return &OAuthAuth{
		config:    config,
		tokenType: "Bearer",
	}
}

// Authenticate performs OAuth authentication.
func (o *OAuthAuth) Authenticate(ctx context.Context, pool *browser.Pool) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.config == nil {
		return fmt.Errorf("OAuth config is required")
	}

	// Try client credentials flow first
	if err := o.clientCredentialsFlow(ctx); err == nil {
		o.authenticated = true
		return nil
	}

	// If we have a browser pool, try authorization code flow
	if pool != nil {
		if err := o.authorizationCodeFlow(ctx, pool); err != nil {
			return err
		}
		o.authenticated = true
		return nil
	}

	return fmt.Errorf("no valid OAuth flow available")
}

// clientCredentialsFlow performs the client credentials grant.
func (o *OAuthAuth) clientCredentialsFlow(ctx context.Context) error {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", o.config.ClientID)
	data.Set("client_secret", o.config.ClientSecret)

	if len(o.config.Scopes) > 0 {
		data.Set("scope", strings.Join(o.config.Scopes, " "))
	}

	return o.requestToken(ctx, data)
}

// authorizationCodeFlow performs the authorization code grant.
func (o *OAuthAuth) authorizationCodeFlow(ctx context.Context, pool *browser.Pool) error {
	// Build authorization URL
	authURL, err := url.Parse(o.config.AuthURL)
	if err != nil {
		return err
	}

	params := authURL.Query()
	params.Set("client_id", o.config.ClientID)
	params.Set("redirect_uri", o.config.RedirectURL)
	params.Set("response_type", "code")
	params.Set("state", generateState())

	if len(o.config.Scopes) > 0 {
		params.Set("scope", strings.Join(o.config.Scopes, " "))
	}

	authURL.RawQuery = params.Encode()

	// Use browser to complete the flow
	result, err := pool.Visit(ctx, authURL.String(), nil, nil)
	if err != nil {
		return err
	}

	// Extract authorization code from redirect
	code := extractAuthCode(result.FinalURL, o.config.RedirectURL)
	if code == "" {
		return fmt.Errorf("failed to obtain authorization code")
	}

	// Exchange code for token
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", o.config.ClientID)
	data.Set("client_secret", o.config.ClientSecret)
	data.Set("redirect_uri", o.config.RedirectURL)

	return o.requestToken(ctx, data)
}

// requestToken exchanges credentials for an access token.
func (o *OAuthAuth) requestToken(ctx context.Context, data url.Values) error {
	req, err := http.NewRequestWithContext(ctx, "POST", o.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	o.accessToken = tokenResp.AccessToken
	o.refreshToken = tokenResp.RefreshToken
	if tokenResp.TokenType != "" {
		o.tokenType = tokenResp.TokenType
	}
	if tokenResp.ExpiresIn > 0 {
		o.expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	return nil
}

// GetHeaders returns the Authorization header.
func (o *OAuthAuth) GetHeaders() map[string]string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.accessToken == "" {
		return nil
	}

	return map[string]string{
		"Authorization": o.tokenType + " " + o.accessToken,
	}
}

// GetCookies returns no cookies for OAuth.
func (o *OAuthAuth) GetCookies() []*http.Cookie {
	return nil
}

// RefreshIfNeeded refreshes the token if needed.
func (o *OAuthAuth) RefreshIfNeeded(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Check if refresh is needed
	if !o.expiry.IsZero() && time.Until(o.expiry) > 5*time.Minute {
		return nil
	}

	if o.refreshToken == "" {
		return nil
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", o.refreshToken)
	data.Set("client_id", o.config.ClientID)
	data.Set("client_secret", o.config.ClientSecret)

	return o.requestToken(ctx, data)
}

// IsAuthenticated returns true if we have a valid token.
func (o *OAuthAuth) IsAuthenticated() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.authenticated || o.accessToken == "" {
		return false
	}

	if !o.expiry.IsZero() && time.Now().After(o.expiry) {
		return false
	}

	return true
}

// Type returns the authentication type.
func (o *OAuthAuth) Type() AuthType {
	return AuthTypeOAuth
}

func generateState() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func extractAuthCode(finalURL, redirectURL string) string {
	parsed, err := url.Parse(finalURL)
	if err != nil {
		return ""
	}

	// Check if we're at the redirect URL
	if !strings.HasPrefix(finalURL, redirectURL) {
		return ""
	}

	return parsed.Query().Get("code")
}
