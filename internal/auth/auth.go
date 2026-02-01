// Package auth provides authentication mechanisms for the crawler.
package auth

import (
	"context"
	"net/http"

	"github.com/PentesterFlow/OpenCrawler/internal/browser"
)

// AuthType represents the type of authentication.
type AuthType string

const (
	AuthTypeNone      AuthType = "none"
	AuthTypeSession   AuthType = "session"
	AuthTypeJWT       AuthType = "jwt"
	AuthTypeOAuth     AuthType = "oauth"
	AuthTypeFormLogin AuthType = "form"
	AuthTypeAPIKey    AuthType = "apikey"
	AuthTypeBasic     AuthType = "basic"
)

// Credentials holds authentication credentials.
type Credentials struct {
	Type        AuthType
	Username    string
	Password    string
	Token       string
	Headers     map[string]string
	Cookies     []*http.Cookie
	LoginURL    string
	FormFields  map[string]string
	OAuthConfig *OAuthConfig
}

// OAuthConfig holds OAuth 2.0 configuration.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RedirectURL  string
	Scopes       []string
}

// Provider defines the interface for authentication providers.
type Provider interface {
	// Authenticate performs authentication
	Authenticate(ctx context.Context, browserPool *browser.Pool) error

	// GetHeaders returns headers to include in requests
	GetHeaders() map[string]string

	// GetCookies returns cookies to include in requests
	GetCookies() []*http.Cookie

	// RefreshIfNeeded refreshes the authentication if needed
	RefreshIfNeeded(ctx context.Context) error

	// IsAuthenticated returns true if currently authenticated
	IsAuthenticated() bool

	// Type returns the authentication type
	Type() AuthType
}

// NewProvider creates an authentication provider based on credentials.
func NewProvider(creds Credentials) (Provider, error) {
	switch creds.Type {
	case AuthTypeNone:
		return &NoAuth{}, nil
	case AuthTypeSession:
		return NewSessionAuth(creds.Cookies), nil
	case AuthTypeJWT:
		return NewJWTAuth(creds.Token), nil
	case AuthTypeOAuth:
		return NewOAuthAuth(creds.OAuthConfig), nil
	case AuthTypeFormLogin:
		return NewFormLoginAuth(creds), nil
	case AuthTypeAPIKey:
		return NewAPIKeyAuth(creds.Headers), nil
	case AuthTypeBasic:
		return NewBasicAuth(creds.Username, creds.Password), nil
	default:
		return &NoAuth{}, nil
	}
}

// NoAuth represents no authentication.
type NoAuth struct{}

func (n *NoAuth) Authenticate(ctx context.Context, pool *browser.Pool) error {
	return nil
}

func (n *NoAuth) GetHeaders() map[string]string {
	return nil
}

func (n *NoAuth) GetCookies() []*http.Cookie {
	return nil
}

func (n *NoAuth) RefreshIfNeeded(ctx context.Context) error {
	return nil
}

func (n *NoAuth) IsAuthenticated() bool {
	return true
}

func (n *NoAuth) Type() AuthType {
	return AuthTypeNone
}
