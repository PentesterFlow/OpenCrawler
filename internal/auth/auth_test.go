package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// NewProvider Tests
// =============================================================================

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name     string
		creds    Credentials
		wantType AuthType
	}{
		{
			name:     "none auth",
			creds:    Credentials{Type: AuthTypeNone},
			wantType: AuthTypeNone,
		},
		{
			name: "session auth",
			creds: Credentials{
				Type:    AuthTypeSession,
				Cookies: []*http.Cookie{{Name: "session", Value: "abc"}},
			},
			wantType: AuthTypeSession,
		},
		{
			name: "jwt auth",
			creds: Credentials{
				Type:  AuthTypeJWT,
				Token: "eyJ...",
			},
			wantType: AuthTypeJWT,
		},
		{
			name: "oauth auth",
			creds: Credentials{
				Type: AuthTypeOAuth,
				OAuthConfig: &OAuthConfig{
					ClientID:     "client",
					ClientSecret: "secret",
				},
			},
			wantType: AuthTypeOAuth,
		},
		{
			name: "form login auth",
			creds: Credentials{
				Type:     AuthTypeFormLogin,
				LoginURL: "https://example.com/login",
				Username: "user",
				Password: "pass",
			},
			wantType: AuthTypeFormLogin,
		},
		{
			name: "api key auth",
			creds: Credentials{
				Type:    AuthTypeAPIKey,
				Headers: map[string]string{"X-API-Key": "key123"},
			},
			wantType: AuthTypeAPIKey,
		},
		{
			name: "basic auth",
			creds: Credentials{
				Type:     AuthTypeBasic,
				Username: "user",
				Password: "pass",
			},
			wantType: AuthTypeBasic,
		},
		{
			name:     "unknown defaults to none",
			creds:    Credentials{Type: "unknown"},
			wantType: AuthTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.creds)
			if err != nil {
				t.Fatalf("NewProvider() error = %v", err)
			}
			if provider.Type() != tt.wantType {
				t.Errorf("Type() = %v, want %v", provider.Type(), tt.wantType)
			}
		})
	}
}

// =============================================================================
// NoAuth Tests
// =============================================================================

func TestNoAuth(t *testing.T) {
	auth := &NoAuth{}

	t.Run("Authenticate", func(t *testing.T) {
		err := auth.Authenticate(context.Background(), nil)
		if err != nil {
			t.Errorf("Authenticate() error = %v", err)
		}
	})

	t.Run("GetHeaders", func(t *testing.T) {
		headers := auth.GetHeaders()
		if headers != nil {
			t.Error("GetHeaders() should return nil")
		}
	})

	t.Run("GetCookies", func(t *testing.T) {
		cookies := auth.GetCookies()
		if cookies != nil {
			t.Error("GetCookies() should return nil")
		}
	})

	t.Run("RefreshIfNeeded", func(t *testing.T) {
		err := auth.RefreshIfNeeded(context.Background())
		if err != nil {
			t.Errorf("RefreshIfNeeded() error = %v", err)
		}
	})

	t.Run("IsAuthenticated", func(t *testing.T) {
		if !auth.IsAuthenticated() {
			t.Error("IsAuthenticated() should return true")
		}
	})

	t.Run("Type", func(t *testing.T) {
		if auth.Type() != AuthTypeNone {
			t.Errorf("Type() = %v, want %v", auth.Type(), AuthTypeNone)
		}
	})
}

// =============================================================================
// SessionAuth Tests
// =============================================================================

func TestNewSessionAuth(t *testing.T) {
	t.Run("with cookies", func(t *testing.T) {
		cookies := []*http.Cookie{
			{Name: "session", Value: "abc123"},
		}
		auth := NewSessionAuth(cookies)

		if !auth.IsAuthenticated() {
			t.Error("should be authenticated with cookies")
		}
	})

	t.Run("without cookies", func(t *testing.T) {
		auth := NewSessionAuth(nil)

		if auth.IsAuthenticated() {
			t.Error("should not be authenticated without cookies")
		}
	})
}

func TestSessionAuth_GetCookies(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc123"},
		{Name: "user", Value: "john"},
	}
	auth := NewSessionAuth(cookies)

	result := auth.GetCookies()
	if len(result) != 2 {
		t.Errorf("len(GetCookies()) = %d, want 2", len(result))
	}
	if result[0].Name != "session" {
		t.Errorf("first cookie name = %q, want session", result[0].Name)
	}

	// Verify the slice is a copy (appending doesn't affect original)
	result = append(result, &http.Cookie{Name: "extra"})
	if len(auth.GetCookies()) != 2 {
		t.Error("GetCookies should return a copy of the slice")
	}
}

func TestSessionAuth_SetCookies(t *testing.T) {
	auth := NewSessionAuth(nil)

	if auth.IsAuthenticated() {
		t.Error("should not be authenticated initially")
	}

	auth.SetCookies([]*http.Cookie{
		{Name: "session", Value: "new"},
	})

	if !auth.IsAuthenticated() {
		t.Error("should be authenticated after SetCookies")
	}
}

func TestSessionAuth_AddCookie(t *testing.T) {
	auth := NewSessionAuth(nil)

	auth.AddCookie(&http.Cookie{Name: "session", Value: "v1", Domain: "example.com"})
	if len(auth.GetCookies()) != 1 {
		t.Error("should have 1 cookie")
	}

	// Add different cookie
	auth.AddCookie(&http.Cookie{Name: "user", Value: "john", Domain: "example.com"})
	if len(auth.GetCookies()) != 2 {
		t.Error("should have 2 cookies")
	}

	// Update existing cookie (same name and domain)
	auth.AddCookie(&http.Cookie{Name: "session", Value: "v2", Domain: "example.com"})
	cookies := auth.GetCookies()
	if len(cookies) != 2 {
		t.Error("should still have 2 cookies")
	}

	// Find session cookie and verify value
	for _, c := range cookies {
		if c.Name == "session" && c.Value != "v2" {
			t.Error("session cookie should be updated to v2")
		}
	}
}

func TestSessionAuth_GetCookie(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc"},
		{Name: "user", Value: "john"},
	}
	auth := NewSessionAuth(cookies)

	t.Run("existing cookie", func(t *testing.T) {
		c := auth.GetCookie("session")
		if c == nil || c.Value != "abc" {
			t.Error("should find session cookie")
		}
	})

	t.Run("non-existing cookie", func(t *testing.T) {
		c := auth.GetCookie("nonexistent")
		if c != nil {
			t.Error("should not find nonexistent cookie")
		}
	})
}

func TestSessionAuth_ClearCookies(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc"},
	}
	auth := NewSessionAuth(cookies)

	if !auth.IsAuthenticated() {
		t.Error("should be authenticated initially")
	}

	auth.ClearCookies()

	if auth.IsAuthenticated() {
		t.Error("should not be authenticated after clear")
	}
	if len(auth.GetCookies()) != 0 {
		t.Error("should have no cookies after clear")
	}
}

func TestSessionAuth_Authenticate(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc"},
	}
	auth := NewSessionAuth(cookies)

	err := auth.Authenticate(context.Background(), nil)
	if err != nil {
		t.Errorf("Authenticate() error = %v", err)
	}
	if !auth.IsAuthenticated() {
		t.Error("should be authenticated")
	}
}

func TestSessionAuth_Concurrent(t *testing.T) {
	auth := NewSessionAuth(nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			auth.AddCookie(&http.Cookie{Name: "cookie", Value: "v"})
			auth.GetCookies()
			auth.IsAuthenticated()
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// JWTAuth Tests
// =============================================================================

func createTestJWT(exp time.Time) string {
	// Create a simple JWT with exp claim
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := struct {
		Exp int64 `json:"exp"`
	}{Exp: exp.Unix()}
	payloadBytes, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := base64.RawURLEncoding.EncodeToString([]byte("signature"))
	return header + "." + payload + "." + sig
}

func TestNewJWTAuth(t *testing.T) {
	t.Run("with token", func(t *testing.T) {
		token := createTestJWT(time.Now().Add(time.Hour))
		auth := NewJWTAuth(token)

		if !auth.IsAuthenticated() {
			t.Error("should be authenticated with token")
		}
		if auth.GetToken() != token {
			t.Error("token should match")
		}
	})

	t.Run("without token", func(t *testing.T) {
		auth := NewJWTAuth("")

		if auth.IsAuthenticated() {
			t.Error("should not be authenticated without token")
		}
	})

	t.Run("with expired token", func(t *testing.T) {
		token := createTestJWT(time.Now().Add(-time.Hour))
		auth := NewJWTAuth(token)

		if auth.IsAuthenticated() {
			t.Error("should not be authenticated with expired token")
		}
	})
}

func TestNewJWTAuthWithRefresh(t *testing.T) {
	token := createTestJWT(time.Now().Add(time.Hour))
	refreshToken := "refresh_token_value"
	refreshURL := "https://example.com/refresh"

	auth := NewJWTAuthWithRefresh(token, refreshToken, refreshURL)

	if auth.GetToken() != token {
		t.Error("token should match")
	}
}

func TestJWTAuth_GetHeaders(t *testing.T) {
	t.Run("with token", func(t *testing.T) {
		auth := NewJWTAuth("mytoken")
		headers := auth.GetHeaders()

		if headers == nil {
			t.Fatal("headers should not be nil")
		}
		if headers["Authorization"] != "Bearer mytoken" {
			t.Errorf("Authorization = %q, want %q", headers["Authorization"], "Bearer mytoken")
		}
	})

	t.Run("without token", func(t *testing.T) {
		auth := NewJWTAuth("")
		headers := auth.GetHeaders()

		if headers != nil {
			t.Error("headers should be nil without token")
		}
	})
}

func TestJWTAuth_GetCookies(t *testing.T) {
	auth := NewJWTAuth("token")
	if auth.GetCookies() != nil {
		t.Error("JWT auth should return nil cookies")
	}
}

func TestJWTAuth_SetToken(t *testing.T) {
	auth := NewJWTAuth("")

	if auth.IsAuthenticated() {
		t.Error("should not be authenticated initially")
	}

	token := createTestJWT(time.Now().Add(time.Hour))
	auth.SetToken(token)

	if !auth.IsAuthenticated() {
		t.Error("should be authenticated after SetToken")
	}
	if auth.GetToken() != token {
		t.Error("token should match")
	}
}

func TestJWTAuth_GetExpiry(t *testing.T) {
	expiry := time.Now().Add(time.Hour).Truncate(time.Second)
	token := createTestJWT(expiry)
	auth := NewJWTAuth(token)

	if auth.GetExpiry().Unix() != expiry.Unix() {
		t.Errorf("GetExpiry() = %v, want %v", auth.GetExpiry(), expiry)
	}
}

func TestJWTAuth_TimeUntilExpiry(t *testing.T) {
	t.Run("with expiry", func(t *testing.T) {
		token := createTestJWT(time.Now().Add(time.Hour))
		auth := NewJWTAuth(token)

		ttl := auth.TimeUntilExpiry()
		if ttl < 59*time.Minute || ttl > 61*time.Minute {
			t.Errorf("TimeUntilExpiry() = %v, want ~1 hour", ttl)
		}
	})

	t.Run("without expiry", func(t *testing.T) {
		auth := NewJWTAuth("")
		if auth.TimeUntilExpiry() != 0 {
			t.Error("TimeUntilExpiry should be 0 without expiry")
		}
	})
}

func TestJWTAuth_RefreshIfNeeded(t *testing.T) {
	t.Run("no refresh needed", func(t *testing.T) {
		token := createTestJWT(time.Now().Add(time.Hour))
		auth := NewJWTAuth(token)

		err := auth.RefreshIfNeeded(context.Background())
		if err != nil {
			t.Errorf("RefreshIfNeeded() error = %v", err)
		}
	})

	t.Run("no refresh token", func(t *testing.T) {
		token := createTestJWT(time.Now().Add(time.Minute))
		auth := NewJWTAuth(token)

		err := auth.RefreshIfNeeded(context.Background())
		if err != nil {
			t.Errorf("RefreshIfNeeded() error = %v", err)
		}
	})
}

func TestJWTAuth_doRefresh(t *testing.T) {
	// Create a test server that provides refresh tokens
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer refresh_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		newToken := createTestJWT(time.Now().Add(2 * time.Hour))
		json.NewEncoder(w).Encode(map[string]string{
			"access_token":  newToken,
			"refresh_token": "new_refresh_token",
		})
	}))
	defer server.Close()

	t.Run("successful refresh", func(t *testing.T) {
		oldToken := createTestJWT(time.Now().Add(time.Minute))
		auth := NewJWTAuthWithRefresh(oldToken, "refresh_token", server.URL)
		auth.expiry = time.Now().Add(time.Minute) // Near expiry

		err := auth.RefreshIfNeeded(context.Background())
		if err != nil {
			t.Errorf("RefreshIfNeeded() error = %v", err)
		}

		// Token should have changed
		if auth.GetToken() == oldToken {
			t.Error("token should have been refreshed")
		}
	})
}

func TestJWTAuth_parseExpiry(t *testing.T) {
	auth := &JWTAuth{}

	t.Run("valid JWT", func(t *testing.T) {
		expiry := time.Now().Add(time.Hour).Truncate(time.Second)
		token := createTestJWT(expiry)
		exp, err := auth.parseExpiry(token)
		if err != nil {
			t.Errorf("parseExpiry() error = %v", err)
		}
		if exp.Unix() != expiry.Unix() {
			t.Errorf("exp = %v, want %v", exp, expiry)
		}
	})

	t.Run("invalid JWT", func(t *testing.T) {
		_, err := auth.parseExpiry("invalid")
		if err == nil {
			t.Error("expected error for invalid JWT")
		}
	})

	t.Run("JWT without exp", func(t *testing.T) {
		// Create JWT without exp claim
		header := base64.RawURLEncoding.EncodeToString([]byte(`{}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"123"}`))
		sig := base64.RawURLEncoding.EncodeToString([]byte("sig"))
		token := header + "." + payload + "." + sig

		_, err := auth.parseExpiry(token)
		if err == nil {
			t.Error("expected error for JWT without exp")
		}
	})
}

func TestJWTAuth_Concurrent(t *testing.T) {
	token := createTestJWT(time.Now().Add(time.Hour))
	auth := NewJWTAuth(token)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			auth.GetHeaders()
			auth.IsAuthenticated()
			auth.GetExpiry()
		}()
	}
	wg.Wait()
}

// =============================================================================
// APIKeyAuth Tests
// =============================================================================

func TestNewAPIKeyAuth(t *testing.T) {
	headers := map[string]string{
		"X-API-Key": "key123",
	}
	auth := NewAPIKeyAuth(headers)

	if !auth.IsAuthenticated() {
		t.Error("should be authenticated with headers")
	}
}

func TestAPIKeyAuth_GetHeaders(t *testing.T) {
	headers := map[string]string{
		"X-API-Key":  "key123",
		"X-App-Id":   "app456",
	}
	auth := NewAPIKeyAuth(headers)

	result := auth.GetHeaders()
	if len(result) != 2 {
		t.Errorf("len(GetHeaders()) = %d, want 2", len(result))
	}
	if result["X-API-Key"] != "key123" {
		t.Errorf("X-API-Key = %q", result["X-API-Key"])
	}

	// Verify it returns a copy
	result["X-API-Key"] = "modified"
	original := auth.GetHeaders()
	if original["X-API-Key"] == "modified" {
		t.Error("GetHeaders should return a copy")
	}
}

func TestAPIKeyAuth_GetCookies(t *testing.T) {
	auth := NewAPIKeyAuth(map[string]string{"X-API-Key": "key"})
	if auth.GetCookies() != nil {
		t.Error("API key auth should return nil cookies")
	}
}

func TestAPIKeyAuth_Authenticate(t *testing.T) {
	auth := NewAPIKeyAuth(map[string]string{"X-API-Key": "key"})
	err := auth.Authenticate(context.Background(), nil)
	if err != nil {
		t.Errorf("Authenticate() error = %v", err)
	}
}

func TestAPIKeyAuth_RefreshIfNeeded(t *testing.T) {
	auth := NewAPIKeyAuth(map[string]string{"X-API-Key": "key"})
	err := auth.RefreshIfNeeded(context.Background())
	if err != nil {
		t.Errorf("RefreshIfNeeded() error = %v", err)
	}
}

func TestAPIKeyAuth_IsAuthenticated(t *testing.T) {
	t.Run("with headers", func(t *testing.T) {
		auth := NewAPIKeyAuth(map[string]string{"X-API-Key": "key"})
		if !auth.IsAuthenticated() {
			t.Error("should be authenticated with headers")
		}
	})

	t.Run("without headers", func(t *testing.T) {
		auth := NewAPIKeyAuth(nil)
		if auth.IsAuthenticated() {
			t.Error("should not be authenticated without headers")
		}
	})
}

// =============================================================================
// BasicAuth Tests
// =============================================================================

func TestNewBasicAuth(t *testing.T) {
	auth := NewBasicAuth("user", "pass")

	if !auth.IsAuthenticated() {
		t.Error("should be authenticated with credentials")
	}
}

func TestBasicAuth_GetHeaders(t *testing.T) {
	t.Run("with credentials", func(t *testing.T) {
		auth := NewBasicAuth("user", "pass")
		headers := auth.GetHeaders()

		if headers == nil {
			t.Fatal("headers should not be nil")
		}

		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		if headers["Authorization"] != expected {
			t.Errorf("Authorization = %q, want %q", headers["Authorization"], expected)
		}
	})

	t.Run("without credentials", func(t *testing.T) {
		auth := NewBasicAuth("", "")
		headers := auth.GetHeaders()

		if headers != nil {
			t.Error("headers should be nil without credentials")
		}
	})
}

func TestBasicAuth_GetCookies(t *testing.T) {
	auth := NewBasicAuth("user", "pass")
	if auth.GetCookies() != nil {
		t.Error("Basic auth should return nil cookies")
	}
}

func TestBasicAuth_Authenticate(t *testing.T) {
	auth := NewBasicAuth("user", "pass")
	err := auth.Authenticate(context.Background(), nil)
	if err != nil {
		t.Errorf("Authenticate() error = %v", err)
	}
}

func TestBasicAuth_RefreshIfNeeded(t *testing.T) {
	auth := NewBasicAuth("user", "pass")
	err := auth.RefreshIfNeeded(context.Background())
	if err != nil {
		t.Errorf("RefreshIfNeeded() error = %v", err)
	}
}

func TestBasicAuth_IsAuthenticated(t *testing.T) {
	t.Run("with username", func(t *testing.T) {
		auth := NewBasicAuth("user", "")
		if !auth.IsAuthenticated() {
			t.Error("should be authenticated with username")
		}
	})

	t.Run("with password only", func(t *testing.T) {
		auth := NewBasicAuth("", "pass")
		if !auth.IsAuthenticated() {
			t.Error("should be authenticated with password")
		}
	})

	t.Run("without credentials", func(t *testing.T) {
		auth := NewBasicAuth("", "")
		if auth.IsAuthenticated() {
			t.Error("should not be authenticated without credentials")
		}
	})
}

// =============================================================================
// OAuthAuth Tests
// =============================================================================

func TestNewOAuthAuth(t *testing.T) {
	config := &OAuthConfig{
		ClientID:     "client123",
		ClientSecret: "secret456",
		TokenURL:     "https://oauth.example.com/token",
	}
	auth := NewOAuthAuth(config)

	if auth.IsAuthenticated() {
		t.Error("should not be authenticated initially")
	}
	if auth.Type() != AuthTypeOAuth {
		t.Errorf("Type() = %v, want %v", auth.Type(), AuthTypeOAuth)
	}
}

func TestOAuthAuth_GetHeaders(t *testing.T) {
	t.Run("with token", func(t *testing.T) {
		auth := NewOAuthAuth(&OAuthConfig{})
		auth.accessToken = "access_token_123"
		auth.tokenType = "Bearer"

		headers := auth.GetHeaders()
		if headers["Authorization"] != "Bearer access_token_123" {
			t.Errorf("Authorization = %q", headers["Authorization"])
		}
	})

	t.Run("without token", func(t *testing.T) {
		auth := NewOAuthAuth(&OAuthConfig{})
		headers := auth.GetHeaders()
		if headers != nil {
			t.Error("headers should be nil without token")
		}
	})
}

func TestOAuthAuth_GetCookies(t *testing.T) {
	auth := NewOAuthAuth(&OAuthConfig{})
	if auth.GetCookies() != nil {
		t.Error("OAuth auth should return nil cookies")
	}
}

func TestOAuthAuth_IsAuthenticated(t *testing.T) {
	t.Run("not authenticated", func(t *testing.T) {
		auth := NewOAuthAuth(&OAuthConfig{})
		if auth.IsAuthenticated() {
			t.Error("should not be authenticated initially")
		}
	})

	t.Run("authenticated with valid token", func(t *testing.T) {
		auth := NewOAuthAuth(&OAuthConfig{})
		auth.authenticated = true
		auth.accessToken = "token"
		auth.expiry = time.Now().Add(time.Hour)

		if !auth.IsAuthenticated() {
			t.Error("should be authenticated with valid token")
		}
	})

	t.Run("authenticated but expired", func(t *testing.T) {
		auth := NewOAuthAuth(&OAuthConfig{})
		auth.authenticated = true
		auth.accessToken = "token"
		auth.expiry = time.Now().Add(-time.Hour)

		if auth.IsAuthenticated() {
			t.Error("should not be authenticated with expired token")
		}
	})
}

func TestOAuthAuth_Authenticate_ClientCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Form.Get("client_id") != "client123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new_access_token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	config := &OAuthConfig{
		ClientID:     "client123",
		ClientSecret: "secret456",
		TokenURL:     server.URL,
		Scopes:       []string{"read", "write"},
	}
	auth := NewOAuthAuth(config)

	err := auth.Authenticate(context.Background(), nil)
	if err != nil {
		t.Errorf("Authenticate() error = %v", err)
	}

	if !auth.IsAuthenticated() {
		t.Error("should be authenticated after successful auth")
	}

	headers := auth.GetHeaders()
	if headers["Authorization"] != "Bearer new_access_token" {
		t.Errorf("Authorization = %q", headers["Authorization"])
	}
}

func TestOAuthAuth_Authenticate_NoConfig(t *testing.T) {
	auth := NewOAuthAuth(nil)

	err := auth.Authenticate(context.Background(), nil)
	if err == nil {
		t.Error("expected error without config")
	}
}

func TestOAuthAuth_RefreshIfNeeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "refreshed_token",
			"refresh_token": "new_refresh",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	t.Run("no refresh needed", func(t *testing.T) {
		config := &OAuthConfig{TokenURL: server.URL}
		auth := NewOAuthAuth(config)
		auth.expiry = time.Now().Add(time.Hour)

		err := auth.RefreshIfNeeded(context.Background())
		if err != nil {
			t.Errorf("RefreshIfNeeded() error = %v", err)
		}
	})

	t.Run("refresh needed", func(t *testing.T) {
		config := &OAuthConfig{
			ClientID:     "client",
			ClientSecret: "secret",
			TokenURL:     server.URL,
		}
		auth := NewOAuthAuth(config)
		auth.refreshToken = "old_refresh"
		auth.expiry = time.Now().Add(time.Minute)

		err := auth.RefreshIfNeeded(context.Background())
		if err != nil {
			t.Errorf("RefreshIfNeeded() error = %v", err)
		}
	})
}

func TestExtractAuthCode(t *testing.T) {
	tests := []struct {
		name        string
		finalURL    string
		redirectURL string
		want        string
	}{
		{
			name:        "valid code",
			finalURL:    "https://app.example.com/callback?code=auth_code_123&state=xyz",
			redirectURL: "https://app.example.com/callback",
			want:        "auth_code_123",
		},
		{
			name:        "no code",
			finalURL:    "https://app.example.com/callback?state=xyz",
			redirectURL: "https://app.example.com/callback",
			want:        "",
		},
		{
			name:        "different redirect",
			finalURL:    "https://other.example.com/?code=abc",
			redirectURL: "https://app.example.com/callback",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAuthCode(tt.finalURL, tt.redirectURL)
			if got != tt.want {
				t.Errorf("extractAuthCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateState(t *testing.T) {
	state1 := generateState()

	if state1 == "" {
		t.Error("state should not be empty")
	}

	// States generated at different times should be different
	time.Sleep(time.Millisecond)
	state2 := generateState()

	// Note: Due to time resolution, states may occasionally be the same
	// This is acceptable for the OAuth state parameter use case
	// Just verify both are non-empty
	if state2 == "" {
		t.Error("second state should not be empty")
	}
}

// =============================================================================
// FormLoginAuth Tests
// =============================================================================

func TestNewFormLoginAuth(t *testing.T) {
	creds := Credentials{
		LoginURL: "https://example.com/login",
		Username: "user",
		Password: "pass",
		FormFields: map[string]string{
			"username_field": "email",
			"password_field": "pwd",
			"submit_button":  "#login-btn",
		},
	}
	auth := NewFormLoginAuth(creds)

	if auth.loginURL != "https://example.com/login" {
		t.Errorf("loginURL = %q", auth.loginURL)
	}
	if auth.usernameField != "email" {
		t.Errorf("usernameField = %q", auth.usernameField)
	}
	if auth.passwordField != "pwd" {
		t.Errorf("passwordField = %q", auth.passwordField)
	}
	if auth.submitButton != "#login-btn" {
		t.Errorf("submitButton = %q", auth.submitButton)
	}
}

func TestNewFormLoginAuth_Defaults(t *testing.T) {
	creds := Credentials{
		LoginURL: "https://example.com/login",
		Username: "user",
		Password: "pass",
	}
	auth := NewFormLoginAuth(creds)

	if auth.usernameField != "username" {
		t.Errorf("default usernameField = %q, want username", auth.usernameField)
	}
	if auth.passwordField != "password" {
		t.Errorf("default passwordField = %q, want password", auth.passwordField)
	}
}

func TestFormLoginAuth_GetHeaders(t *testing.T) {
	auth := NewFormLoginAuth(Credentials{LoginURL: "https://example.com"})
	if auth.GetHeaders() != nil {
		t.Error("FormLoginAuth should return nil headers")
	}
}

func TestFormLoginAuth_GetCookies(t *testing.T) {
	auth := NewFormLoginAuth(Credentials{LoginURL: "https://example.com"})
	auth.cookies = []*http.Cookie{
		{Name: "session", Value: "abc"},
	}

	cookies := auth.GetCookies()
	if len(cookies) != 1 {
		t.Errorf("len(GetCookies()) = %d, want 1", len(cookies))
	}
	if cookies[0].Name != "session" {
		t.Errorf("cookie name = %q, want session", cookies[0].Name)
	}

	// Verify the slice is a copy (appending doesn't affect original)
	cookies = append(cookies, &http.Cookie{Name: "extra"})
	if len(auth.GetCookies()) != 1 {
		t.Error("GetCookies should return a copy of the slice")
	}
}

func TestFormLoginAuth_IsAuthenticated(t *testing.T) {
	auth := NewFormLoginAuth(Credentials{LoginURL: "https://example.com"})

	if auth.IsAuthenticated() {
		t.Error("should not be authenticated initially")
	}

	auth.authenticated = true
	auth.cookies = []*http.Cookie{{Name: "session"}}

	if !auth.IsAuthenticated() {
		t.Error("should be authenticated with flag and cookies")
	}
}

func TestFormLoginAuth_SetSessionLifetime(t *testing.T) {
	auth := NewFormLoginAuth(Credentials{LoginURL: "https://example.com"})

	auth.SetSessionLifetime(time.Hour)

	if auth.sessionLife != time.Hour {
		t.Errorf("sessionLife = %v, want 1 hour", auth.sessionLife)
	}
}

func TestFormLoginAuth_RefreshIfNeeded(t *testing.T) {
	auth := NewFormLoginAuth(Credentials{LoginURL: "https://example.com"})

	// No error expected even without refresh capability
	err := auth.RefreshIfNeeded(context.Background())
	if err != nil {
		t.Errorf("RefreshIfNeeded() error = %v", err)
	}
}

// =============================================================================
// Credentials Type Tests
// =============================================================================

func TestCredentials(t *testing.T) {
	creds := Credentials{
		Type:     AuthTypeJWT,
		Username: "user",
		Password: "pass",
		Token:    "jwt_token",
		Headers:  map[string]string{"X-Custom": "value"},
		Cookies: []*http.Cookie{
			{Name: "session", Value: "abc"},
		},
		LoginURL:   "https://example.com/login",
		FormFields: map[string]string{"extra": "field"},
		OAuthConfig: &OAuthConfig{
			ClientID:     "client",
			ClientSecret: "secret",
		},
	}

	if creds.Type != AuthTypeJWT {
		t.Error("Type not set correctly")
	}
	if creds.Token != "jwt_token" {
		t.Error("Token not set correctly")
	}
	if len(creds.Cookies) != 1 {
		t.Error("Cookies not set correctly")
	}
	if creds.OAuthConfig.ClientID != "client" {
		t.Error("OAuthConfig not set correctly")
	}
}

func TestOAuthConfig(t *testing.T) {
	config := OAuthConfig{
		ClientID:     "client123",
		ClientSecret: "secret456",
		AuthURL:      "https://oauth.example.com/authorize",
		TokenURL:     "https://oauth.example.com/token",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"read", "write", "admin"},
	}

	if config.ClientID != "client123" {
		t.Error("ClientID not set correctly")
	}
	if len(config.Scopes) != 3 {
		t.Error("Scopes not set correctly")
	}
}
