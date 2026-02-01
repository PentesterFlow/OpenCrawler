package crawler

import (
	"io"
	"net/http"
	"time"

	"github.com/PentesterFlow/OpenCrawler/internal/logger"
	"github.com/PentesterFlow/OpenCrawler/internal/metrics"
)

// Option is a functional option for configuring the Crawler.
type Option func(*Crawler) error

// WithTarget sets the target URL to crawl.
func WithTarget(url string) Option {
	return func(c *Crawler) error {
		c.config.Target = url
		return nil
	}
}

// WithWorkers sets the number of concurrent workers.
func WithWorkers(n int) Option {
	return func(c *Crawler) error {
		if n < 1 {
			n = 1
		}
		c.config.Workers = n
		return nil
	}
}

// WithMaxDepth sets the maximum crawl depth.
func WithMaxDepth(depth int) Option {
	return func(c *Crawler) error {
		if depth < 1 {
			depth = 1
		}
		c.config.MaxDepth = depth
		c.config.Scope.MaxDepth = depth
		return nil
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Crawler) error {
		c.config.Timeout = timeout
		return nil
	}
}

// WithScope sets the scope rules.
func WithScope(scope ScopeRules) Option {
	return func(c *Crawler) error {
		c.config.Scope = scope
		return nil
	}
}

// WithIncludePatterns adds URL patterns to include.
func WithIncludePatterns(patterns ...string) Option {
	return func(c *Crawler) error {
		c.config.Scope.IncludePatterns = append(c.config.Scope.IncludePatterns, patterns...)
		return nil
	}
}

// WithExcludePatterns adds URL patterns to exclude.
func WithExcludePatterns(patterns ...string) Option {
	return func(c *Crawler) error {
		c.config.Scope.ExcludePatterns = append(c.config.Scope.ExcludePatterns, patterns...)
		return nil
	}
}

// WithAllowedDomains sets the allowed domains.
func WithAllowedDomains(domains ...string) Option {
	return func(c *Crawler) error {
		c.config.Scope.AllowedDomains = append(c.config.Scope.AllowedDomains, domains...)
		return nil
	}
}

// WithFollowExternal enables following external links.
func WithFollowExternal(follow bool) Option {
	return func(c *Crawler) error {
		c.config.Scope.FollowExternal = follow
		return nil
	}
}

// WithRateLimit sets the rate limiting configuration.
func WithRateLimit(rps float64, burst int) Option {
	return func(c *Crawler) error {
		c.config.RateLimit.RequestsPerSecond = rps
		c.config.RateLimit.Burst = burst
		return nil
	}
}

// WithRespectRobotsTxt enables/disables robots.txt respect.
func WithRespectRobotsTxt(respect bool) Option {
	return func(c *Crawler) error {
		c.config.RateLimit.RespectRobotsTxt = respect
		return nil
	}
}

// WithBrowserPool sets the browser pool size.
func WithBrowserPool(size int) Option {
	return func(c *Crawler) error {
		if size < 1 {
			size = 1
		}
		c.config.Browser.PoolSize = size
		return nil
	}
}

// WithHeadless enables/disables headless mode.
func WithHeadless(headless bool) Option {
	return func(c *Crawler) error {
		c.config.Browser.Headless = headless
		return nil
	}
}

// WithUserAgent sets the user agent string.
func WithUserAgent(ua string) Option {
	return func(c *Crawler) error {
		c.config.Browser.UserAgent = ua
		return nil
	}
}

// WithProxy sets the proxy URL.
func WithProxy(proxy string) Option {
	return func(c *Crawler) error {
		c.config.Proxy = proxy
		return nil
	}
}

// FormAuth holds form-based authentication configuration.
type FormAuth struct {
	LoginURL   string
	Username   string
	Password   string
	UsernameField string
	PasswordField string
	ExtraFields map[string]string
}

// WithAuth sets authentication credentials.
func WithAuth(auth AuthCredentials) Option {
	return func(c *Crawler) error {
		c.config.Auth = auth
		return nil
	}
}

// WithFormAuth configures form-based authentication.
func WithFormAuth(auth FormAuth) Option {
	return func(c *Crawler) error {
		c.config.Auth = AuthCredentials{
			Type:     AuthTypeFormLogin,
			LoginURL: auth.LoginURL,
			Username: auth.Username,
			Password: auth.Password,
			FormFields: map[string]string{
				"username_field": auth.UsernameField,
				"password_field": auth.PasswordField,
			},
		}
		if auth.ExtraFields != nil {
			for k, v := range auth.ExtraFields {
				c.config.Auth.FormFields[k] = v
			}
		}
		return nil
	}
}

// WithJWTAuth configures JWT authentication.
func WithJWTAuth(token string) Option {
	return func(c *Crawler) error {
		c.config.Auth = AuthCredentials{
			Type:  AuthTypeJWT,
			Token: token,
		}
		return nil
	}
}

// WithBasicAuth configures basic authentication.
func WithBasicAuth(username, password string) Option {
	return func(c *Crawler) error {
		c.config.Auth = AuthCredentials{
			Type:     AuthTypeBasic,
			Username: username,
			Password: password,
		}
		return nil
	}
}

// WithAPIKeyAuth configures API key authentication.
func WithAPIKeyAuth(headerName, apiKey string) Option {
	return func(c *Crawler) error {
		c.config.Auth = AuthCredentials{
			Type: AuthTypeAPIKey,
			Headers: map[string]string{
				headerName: apiKey,
			},
		}
		return nil
	}
}

// WithCookies sets cookies to include in requests.
func WithCookies(cookies []*http.Cookie) Option {
	return func(c *Crawler) error {
		c.config.Auth.Cookies = cookies
		if c.config.Auth.Type == AuthTypeNone {
			c.config.Auth.Type = AuthTypeSession
		}
		return nil
	}
}

// WithCustomHeaders sets custom headers for all requests.
func WithCustomHeaders(headers map[string]string) Option {
	return func(c *Crawler) error {
		if c.config.CustomHeaders == nil {
			c.config.CustomHeaders = make(map[string]string)
		}
		for k, v := range headers {
			c.config.CustomHeaders[k] = v
		}
		return nil
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) Option {
	return func(c *Crawler) error {
		c.outputWriter = w
		return nil
	}
}

// WithOutputFile sets the output file path.
func WithOutputFile(path string) Option {
	return func(c *Crawler) error {
		c.config.Output.FilePath = path
		return nil
	}
}

// WithPrettyOutput enables/disables pretty JSON output.
func WithPrettyOutput(pretty bool) Option {
	return func(c *Crawler) error {
		c.config.Output.Pretty = pretty
		return nil
	}
}

// WithStreamMode enables streaming output mode.
func WithStreamMode(stream bool) Option {
	return func(c *Crawler) error {
		c.config.Output.StreamMode = stream
		return nil
	}
}

// WithStateFile sets the state file path for persistence.
func WithStateFile(path string) Option {
	return func(c *Crawler) error {
		c.config.State.FilePath = path
		c.config.State.Enabled = true
		return nil
	}
}

// WithAutoSave enables/disables automatic state saving.
func WithAutoSave(enabled bool, intervalSeconds int) Option {
	return func(c *Crawler) error {
		c.config.State.AutoSave = enabled
		c.config.State.Interval = intervalSeconds
		return nil
	}
}

// WithPassiveDiscovery enables/disables passive API discovery.
func WithPassiveDiscovery(enabled bool) Option {
	return func(c *Crawler) error {
		c.config.PassiveAPIDiscovery = enabled
		return nil
	}
}

// WithActiveDiscovery enables/disables active API probing.
func WithActiveDiscovery(enabled bool) Option {
	return func(c *Crawler) error {
		c.config.ActiveAPIDiscovery = enabled
		return nil
	}
}

// WithWebSocketDiscovery enables/disables WebSocket discovery.
func WithWebSocketDiscovery(enabled bool) Option {
	return func(c *Crawler) error {
		c.config.WebSocketDiscovery = enabled
		return nil
	}
}

// WithFormAnalysis enables/disables form analysis.
func WithFormAnalysis(enabled bool) Option {
	return func(c *Crawler) error {
		c.config.FormAnalysis = enabled
		return nil
	}
}

// WithJSAnalysis enables/disables JavaScript analysis.
func WithJSAnalysis(enabled bool) Option {
	return func(c *Crawler) error {
		c.config.JSAnalysis = enabled
		return nil
	}
}

// WithVerbose enables/disables verbose logging.
func WithVerbose(verbose bool) Option {
	return func(c *Crawler) error {
		c.config.Verbose = verbose
		return nil
	}
}

// WithDebug enables/disables debug mode.
func WithDebug(debug bool) Option {
	return func(c *Crawler) error {
		c.config.Debug = debug
		return nil
	}
}

// WithLogger sets a custom logger.
func WithLogger(l *logger.Logger) Option {
	return func(c *Crawler) error {
		c.logger = l
		return nil
	}
}

// WithLogLevel sets the log level.
func WithLogLevel(level logger.Level) Option {
	return func(c *Crawler) error {
		if c.logger != nil {
			c.logger.SetLevel(level)
		}
		return nil
	}
}

// WithMetrics sets a custom metrics collector.
func WithMetrics(m *metrics.Collector) Option {
	return func(c *Crawler) error {
		c.metrics = m
		return nil
	}
}

// WithConfig sets the entire configuration.
func WithConfig(config *Config) Option {
	return func(c *Crawler) error {
		c.config = config
		return nil
	}
}

// WithProgress enables/disables progress bar display.
func WithProgress(enabled bool) Option {
	return func(c *Crawler) error {
		c.showProgress = enabled
		return nil
	}
}
