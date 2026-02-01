package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/PentesterFlow/OpenCrawler/pkg/crawler"
)

var (
	version = "1.0.0"

	// Global flags
	configFile string
	verbose    bool
	debug      bool

	// Crawl flags
	workers       int
	maxDepth      int
	timeout       int
	rateLimit     float64
	browserPool   int
	outputFile    string
	stateFile     string
	includePatterns []string
	excludePatterns []string

	// Auth flags
	authType      string
	loginURL      string
	username      string
	password      string
	token         string
	apiKeyHeader  string
	apiKey        string

	// Feature flags
	noPassiveAPI  bool
	noActiveAPI   bool
	noWebSocket   bool
	noForms       bool
	noJS          bool
	respectRobots bool
	followExternal bool

	// Performance flags
	turboMode    bool
	balancedMode bool

	// Display flags
	showProgress bool
	noProgress   bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "opencrawler",
		Short: "OpenCrawler - Web Application Crawler",
		Long: `OpenCrawler - A high-performance web application crawler for security testing.

Supports HTML pages, Single Page Applications (SPAs), REST APIs, GraphQL, and WebSockets.
Features headless browser rendering, authentication support, and comprehensive endpoint discovery.`,
		Version: version,
	}

	// Crawl command
	crawlCmd := &cobra.Command{
		Use:   "crawl [target]",
		Short: "Crawl a target URL",
		Long:  "Crawl a target URL and discover endpoints, forms, and APIs.",
		Args:  cobra.ExactArgs(1),
		RunE:  runCrawl,
	}

	// Resume command
	resumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume an interrupted crawl",
		Long:  "Resume a previously interrupted crawl from a saved state file.",
		RunE:  runResume,
	}

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show crawl status",
		Long:  "Show the status of a running or saved crawl.",
		RunE:  runStatus,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Debug mode")

	// Crawl flags
	crawlCmd.Flags().IntVarP(&workers, "workers", "w", 50, "Number of concurrent workers")
	crawlCmd.Flags().IntVarP(&maxDepth, "max-depth", "d", 10, "Maximum crawl depth")
	crawlCmd.Flags().IntVarP(&timeout, "timeout", "t", 30, "Request timeout in seconds")
	crawlCmd.Flags().Float64VarP(&rateLimit, "rate-limit", "r", 100, "Requests per second")
	crawlCmd.Flags().IntVar(&browserPool, "browser-pool", 10, "Browser pool size")
	crawlCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	crawlCmd.Flags().StringVar(&stateFile, "state-file", "", "State file for persistence")
	crawlCmd.Flags().StringArrayVar(&includePatterns, "include", nil, "URL patterns to include (regex)")
	crawlCmd.Flags().StringArrayVar(&excludePatterns, "exclude", nil, "URL patterns to exclude (regex)")

	// Auth flags
	crawlCmd.Flags().StringVar(&authType, "auth-type", "none", "Authentication type (none, form, jwt, basic, apikey)")
	crawlCmd.Flags().StringVar(&loginURL, "login-url", "", "Login URL for form authentication")
	crawlCmd.Flags().StringVarP(&username, "username", "u", "", "Username for authentication")
	crawlCmd.Flags().StringVarP(&password, "password", "p", "", "Password for authentication")
	crawlCmd.Flags().StringVar(&token, "token", "", "JWT or Bearer token")
	crawlCmd.Flags().StringVar(&apiKeyHeader, "api-key-header", "X-API-Key", "API key header name")
	crawlCmd.Flags().StringVar(&apiKey, "api-key", "", "API key value")

	// Feature flags
	crawlCmd.Flags().BoolVar(&noPassiveAPI, "no-passive-api", false, "Disable passive API discovery")
	crawlCmd.Flags().BoolVar(&noActiveAPI, "no-active-api", false, "Disable active API probing")
	crawlCmd.Flags().BoolVar(&noWebSocket, "no-websocket", false, "Disable WebSocket discovery")
	crawlCmd.Flags().BoolVar(&noForms, "no-forms", false, "Disable form analysis")
	crawlCmd.Flags().BoolVar(&noJS, "no-js", false, "Disable JavaScript analysis")
	crawlCmd.Flags().BoolVar(&respectRobots, "respect-robots", true, "Respect robots.txt")
	crawlCmd.Flags().BoolVar(&followExternal, "follow-external", false, "Follow external links")

	// Performance flags
	crawlCmd.Flags().BoolVar(&turboMode, "turbo", false, "TURBO MODE: Maximum speed (200 workers, fast HTTP, minimal analysis)")
	crawlCmd.Flags().BoolVar(&balancedMode, "balanced", false, "Balanced mode: Good speed with thorough discovery")

	// Display flags
	crawlCmd.Flags().BoolVar(&showProgress, "progress", true, "Show progress bar during crawling")
	crawlCmd.Flags().BoolVar(&noProgress, "no-progress", false, "Disable progress bar (use verbose logging instead)")

	// Resume flags
	resumeCmd.Flags().StringVar(&stateFile, "state-file", "", "State file to resume from")
	resumeCmd.MarkFlagRequired("state-file")

	// Status flags
	statusCmd.Flags().StringVar(&stateFile, "state-file", "", "State file to check")

	// Add commands
	rootCmd.AddCommand(crawlCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(statusCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runCrawl(cmd *cobra.Command, args []string) error {
	target := args[0]

	// Build configuration based on mode
	var config *crawler.Config
	if turboMode {
		config = crawler.TurboConfig()
		fmt.Println("⚡ TURBO MODE ENABLED - Maximum Speed!")
	} else if balancedMode {
		config = crawler.BalancedConfig()
		fmt.Println("⚖️  Balanced Mode - Speed + Thoroughness")
	} else {
		config = crawler.DefaultConfig()
	}

	config.Target = target

	// Override with command-line flags if provided
	if cmd.Flags().Changed("workers") {
		config.Workers = workers
	}
	if cmd.Flags().Changed("max-depth") {
		config.MaxDepth = maxDepth
		config.Scope.MaxDepth = maxDepth
	}
	if cmd.Flags().Changed("timeout") {
		config.Timeout = time.Duration(timeout) * time.Second
	}
	if cmd.Flags().Changed("rate-limit") {
		config.RateLimit.RequestsPerSecond = rateLimit
	}
	if cmd.Flags().Changed("browser-pool") {
		config.Browser.PoolSize = browserPool
	}

	config.Scope.FollowExternal = followExternal
	config.Scope.IncludePatterns = includePatterns
	config.Scope.ExcludePatterns = excludePatterns
	config.RateLimit.RespectRobotsTxt = respectRobots
	config.Output.FilePath = outputFile
	config.State.FilePath = stateFile

	// Only apply feature flags if not in turbo mode (turbo has its own settings)
	if !turboMode {
		config.PassiveAPIDiscovery = !noPassiveAPI
		config.ActiveAPIDiscovery = !noActiveAPI
		config.WebSocketDiscovery = !noWebSocket
		config.FormAnalysis = !noForms
		config.JSAnalysis = !noJS
	}
	config.Verbose = verbose
	config.Debug = debug

	// Configure authentication
	switch authType {
	case "form":
		config.Auth = crawler.AuthCredentials{
			Type:     crawler.AuthTypeFormLogin,
			LoginURL: loginURL,
			Username: username,
			Password: password,
		}
	case "jwt":
		config.Auth = crawler.AuthCredentials{
			Type:  crawler.AuthTypeJWT,
			Token: token,
		}
	case "basic":
		config.Auth = crawler.AuthCredentials{
			Type:     crawler.AuthTypeBasic,
			Username: username,
			Password: password,
		}
	case "apikey":
		config.Auth = crawler.AuthCredentials{
			Type: crawler.AuthTypeAPIKey,
			Headers: map[string]string{
				apiKeyHeader: apiKey,
			},
		}
	}

	// Load config file if provided
	if configFile != "" {
		fileConfig, err := crawler.LoadFromFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
		// Merge with command-line options (command-line takes precedence)
		if target != "" {
			fileConfig.Target = target
		}
		config = fileConfig
	}

	// Determine if progress bar should be shown
	enableProgress := showProgress && !noProgress && !verbose

	// Create crawler with progress option
	c, err := crawler.New(
		crawler.WithConfig(config),
		crawler.WithProgress(enableProgress),
	)
	if err != nil {
		return fmt.Errorf("failed to create crawler: %w", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived interrupt signal, stopping...\n")
		cancel()

		// Save state if configured
		if stateFile != "" {
			if err := c.SaveState(stateFile); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "State saved to %s\n", stateFile)
			}
		}
	}()

	// Print banner (only if not showing progress bar)
	if !enableProgress {
		printBanner(target, config)
	} else {
		// Print minimal header for progress mode
		fmt.Println()
		fmt.Println("OpenCrawler v1.0 - Starting crawl...")
		fmt.Printf("Target: %s\n", target)
		fmt.Println()
	}

	// Start crawling
	startTime := time.Now()
	result, err := c.Start(ctx)
	duration := time.Since(startTime)

	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("crawl failed: %w", err)
	}

	// Print summary (only if progress bar is disabled - progress bar prints its own summary)
	if result != nil && !enableProgress {
		printSummary(result, duration)
	}

	return nil
}

func runResume(cmd *cobra.Command, args []string) error {
	// Create crawler with default config
	c, err := crawler.New()
	if err != nil {
		return fmt.Errorf("failed to create crawler: %w", err)
	}

	// Load saved state
	if err := c.LoadState(stateFile); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	fmt.Printf("Resuming crawl from %s\n", stateFile)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived interrupt signal, stopping...\n")
		cancel()
		c.SaveState(stateFile)
	}()

	// Resume crawling
	result, err := c.Start(ctx)
	if err != nil {
		return fmt.Errorf("crawl failed: %w", err)
	}

	printSummary(result, result.CompletedAt.Sub(result.StartedAt))

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	if stateFile == "" {
		fmt.Println("No state file specified")
		return nil
	}

	// This would read and display state file info
	fmt.Printf("State file: %s\n", stateFile)
	fmt.Println("Status: Implementation pending")

	return nil
}

func printBanner(target string, config *crawler.Config) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      OpenCrawler v1.0                        ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Target:     %s\n", target)
	fmt.Printf("Workers:    %d\n", config.Workers)
	fmt.Printf("Max Depth:  %d\n", config.MaxDepth)
	fmt.Printf("Rate Limit: %.0f req/s\n", config.RateLimit.RequestsPerSecond)
	fmt.Println()
	fmt.Println("Starting crawl...")
	fmt.Println()
}

func printSummary(result *crawler.CrawlResult, duration time.Duration) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                       Crawl Summary                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Duration:           %v\n", duration.Round(time.Second))
	fmt.Printf("URLs Discovered:    %d\n", result.Stats.URLsDiscovered)
	fmt.Printf("Pages Crawled:      %d\n", result.Stats.PagesCrawled)
	fmt.Printf("Forms Found:        %d\n", result.Stats.FormsFound)
	fmt.Printf("API Endpoints:      %d\n", result.Stats.APIEndpoints)
	fmt.Printf("WebSocket Endpoints:%d\n", result.Stats.WebSocketEndpoints)
	fmt.Printf("Errors:             %d\n", result.Stats.ErrorCount)
	fmt.Println()

	if len(result.Endpoints) > 0 {
		fmt.Println("Top Discovered Endpoints:")
		count := 10
		if len(result.Endpoints) < count {
			count = len(result.Endpoints)
		}
		for i := 0; i < count; i++ {
			ep := result.Endpoints[i]
			fmt.Printf("  [%s] %s\n", ep.Method, ep.URL)
		}
		if len(result.Endpoints) > 10 {
			fmt.Printf("  ... and %d more\n", len(result.Endpoints)-10)
		}
		fmt.Println()
	}

	if len(result.Forms) > 0 {
		fmt.Println("Discovered Forms:")
		count := 5
		if len(result.Forms) < count {
			count = len(result.Forms)
		}
		for i := 0; i < count; i++ {
			form := result.Forms[i]
			fmt.Printf("  [%s] %s -> %s\n", form.Method, form.URL, form.Action)
		}
		if len(result.Forms) > 5 {
			fmt.Printf("  ... and %d more\n", len(result.Forms)-5)
		}
		fmt.Println()
	}

	if len(result.WebSockets) > 0 {
		fmt.Println("WebSocket Endpoints:")
		for _, ws := range result.WebSockets {
			fmt.Printf("  %s\n", ws.URL)
		}
		fmt.Println()
	}
}
