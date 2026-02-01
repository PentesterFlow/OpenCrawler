# OpenCrawler

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-blue?style=for-the-badge" alt="Platform">
</p>

<p align="center">
  <b>A high-performance web application crawler for security testing and reconnaissance</b>
</p>

OpenCrawler is designed for Dynamic Application Security Testing (DAST). Built in Go with headless browser support, it discovers endpoints, forms, APIs, and WebSocket connections across modern web applications including Single Page Applications (SPAs).

## Features

- **High Performance**: Concurrent crawling with 100+ workers, achieving 50-100+ pages/second
- **SPA Support**: Full support for React, Angular, Vue, Ember, and other JavaScript frameworks
- **Headless Browser**: Chrome/Chromium integration via CDP for JavaScript rendering
- **API Discovery**: Passive (XHR interception) and active (endpoint probing) API discovery
- **WebSocket Support**: Automatic WebSocket endpoint detection and message recording
- **Form Analysis**: Comprehensive form detection with CSRF token identification
- **Authentication**: Support for JWT, OAuth, Basic Auth, API Keys, and form-based login
- **Smart Deduplication**: Bloom filter-based URL deduplication for memory efficiency
- **Real-time Progress**: Live progress bar with crawl statistics
- **State Persistence**: Save and resume interrupted crawls
- **Configurable Scope**: Regex-based include/exclude patterns, domain restrictions

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/PentesterFlow/OpenCrawler.git
cd OpenCrawler

# Build
go build -o opencrawler ./cmd/crawler

# Or install directly
go install github.com/PentesterFlow/OpenCrawler/cmd/crawler@latest
```

### Requirements

- Go 1.21 or higher
- Chrome/Chromium (for headless browser features)

## Quick Start

```bash
# Basic crawl
./opencrawler crawl https://example.com

# Turbo mode (maximum speed)
./opencrawler crawl https://example.com --turbo

# With depth limit
./opencrawler crawl https://example.com --max-depth 5

# Save results to file
./opencrawler crawl https://example.com -o results.json
```

## Usage

### Basic Options

```bash
./opencrawler crawl [target] [flags]

Flags:
  -w, --workers int         Number of concurrent workers (default 50)
  -d, --max-depth int       Maximum crawl depth (default 10)
  -t, --timeout int         Request timeout in seconds (default 30)
  -r, --rate-limit float    Requests per second (default 100)
  -o, --output string       Output file (default: stdout)
      --progress            Show progress bar (default true)
  -v, --verbose             Verbose logging
```

### Performance Modes

```bash
# Turbo Mode - Maximum speed (200 workers, minimal analysis)
./opencrawler crawl https://example.com --turbo

# Balanced Mode - Good speed with thorough discovery
./opencrawler crawl https://example.com --balanced
```

### Authentication

```bash
# JWT/Bearer Token
./opencrawler crawl https://example.com --auth-type jwt --token "your-token"

# Basic Auth
./opencrawler crawl https://example.com --auth-type basic -u admin -p secret

# API Key
./opencrawler crawl https://example.com --auth-type apikey --api-key-header "X-API-Key" --api-key "your-key"

# Form Login
./opencrawler crawl https://example.com --auth-type form --login-url https://example.com/login -u admin -p secret
```

### Scope Control

```bash
# Include only API paths
./opencrawler crawl https://example.com --include ".*api.*" --include ".*v1.*"

# Exclude certain paths
./opencrawler crawl https://example.com --exclude ".*logout.*" --exclude ".*admin.*"

# Follow external links
./opencrawler crawl https://example.com --follow-external
```

### Feature Toggles

```bash
# Disable specific features
./opencrawler crawl https://example.com \
  --no-passive-api \    # Disable passive API discovery
  --no-active-api \     # Disable active API probing
  --no-websocket \      # Disable WebSocket discovery
  --no-forms \          # Disable form analysis
  --no-js               # Disable JavaScript analysis
```

### State Persistence

```bash
# Save state for resume
./opencrawler crawl https://example.com --state-file crawl-state.db

# Resume interrupted crawl
./opencrawler resume --state-file crawl-state.db
```

## Progress Display

OpenCrawler shows real-time progress during crawling:

```
[██████████████████████████████] 100% | Pages: 746 | Queue: 0 | APIs: 12 | Forms: 3 | 80.0 p/s | 9s
```

Upon completion:

```
╔══════════════════════════════════════════════════════════════╗
║                       Crawl Complete                         ║
╚══════════════════════════════════════════════════════════════╝

  Target:              https://example.com
  Duration:            2m15s
  URLs Discovered:     1500
  Pages Crawled:       450
  Forms Found:         23
  API Endpoints:       87
  WebSocket Endpoints: 3
  Errors:              5

  Average Speed:       3.3 pages/sec
```

## Output Format

Results are output in JSON format:

```json
{
  "target": "https://example.com",
  "started_at": "2024-01-15T10:00:00Z",
  "completed_at": "2024-01-15T10:15:00Z",
  "stats": {
    "urls_discovered": 1500,
    "pages_crawled": 450,
    "forms_found": 23,
    "api_endpoints": 87,
    "websocket_endpoints": 3
  },
  "endpoints": [...],
  "forms": [...],
  "websockets": [...]
}
```

## Configuration File

Create a YAML or JSON configuration file:

```yaml
# config.yaml
target: https://example.com
workers: 100
max_depth: 15
timeout: 30s

scope:
  include_patterns:
    - ".*api.*"
  exclude_patterns:
    - ".*logout.*"
  max_depth: 15

rate_limit:
  requests_per_second: 100
  burst: 20
  respect_robots_txt: true

auth:
  type: jwt
  token: "your-token"

output:
  format: json
  file_path: results.json
  pretty: true
```

Use with:

```bash
./opencrawler crawl https://example.com -c config.yaml
```

## Library Usage

OpenCrawler can be used as a Go library:

```go
package main

import (
    "context"
    "fmt"
    "github.com/PentesterFlow/OpenCrawler/pkg/crawler"
)

func main() {
    c, err := crawler.New(
        crawler.WithTarget("https://example.com"),
        crawler.WithWorkers(50),
        crawler.WithMaxDepth(10),
        crawler.WithJWTAuth("your-token"),
        crawler.WithProgress(true),
    )
    if err != nil {
        panic(err)
    }

    result, err := c.Start(context.Background())
    if err != nil {
        panic(err)
    }

    fmt.Printf("Crawled %d pages\n", result.Stats.PagesCrawled)
    fmt.Printf("Found %d API endpoints\n", result.Stats.APIEndpoints)
}
```

## Architecture

```
OpenCrawler/
├── cmd/crawler/          # CLI entry point
├── pkg/crawler/          # Public API (crawler, config, options)
└── internal/
    ├── auth/             # Authentication providers
    ├── browser/          # Chrome CDP integration
    ├── discovery/        # API discovery (passive/active)
    ├── framework/        # SPA framework detection
    ├── http/             # Fast HTTP client
    ├── parser/           # HTML/JS/Form parsing
    ├── progress/         # Progress bar display
    ├── queue/            # URL queue management
    ├── scope/            # Scope checking
    ├── state/            # State persistence
    └── websocket/        # WebSocket handling
```

## Performance Tips

1. **Use Turbo Mode** for large sites: `--turbo`
2. **Limit depth** for focused crawls: `--max-depth 3`
3. **Increase workers** for fast sites: `-w 200`
4. **Disable unused features**: `--no-js --no-websocket`
5. **Use state persistence** for long crawls: `--state-file state.db`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Disclaimer

This tool is intended for authorized security testing and educational purposes only. Always obtain proper authorization before crawling or testing any web application. The authors are not responsible for any misuse of this tool.

## Acknowledgments

- [go-rod](https://github.com/go-rod/rod) - Headless browser automation
- [goquery](https://github.com/PuerkitoBio/goquery) - HTML parsing
- [cobra](https://github.com/spf13/cobra) - CLI framework
- [bbolt](https://github.com/etcd-io/bbolt) - State persistence

---

<p align="center">Made with ❤️ by <a href="https://github.com/PentesterFlow">PentesterFlow</a></p>
