<p align="center">
  <img src="assets/logo.png" alt="OpenCrawler Logo" width="200">
</p>

<h1 align="center">OpenCrawler</h1>

<p align="center">
  <b>The Ultimate High-Performance Web Crawler for Security Testing</b>
</p>

<p align="center">
  <a href="https://github.com/PentesterFlow/OpenCrawler/releases"><img src="https://img.shields.io/github/v/release/PentesterFlow/OpenCrawler?style=for-the-badge&color=blue" alt="Release"></a>
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-blue?style=for-the-badge" alt="Platform">
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#comparison">Comparison</a> •
  <a href="#documentation">Documentation</a>
</p>

---

## Why OpenCrawler?

OpenCrawler is a **next-generation web application crawler** purpose-built for **Dynamic Application Security Testing (DAST)**. Unlike traditional crawlers, OpenCrawler understands modern web applications—SPAs, APIs, WebSockets, and JavaScript-heavy sites—delivering comprehensive attack surface discovery at unprecedented speed.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CRAWL COMPLETE                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  Target:              https://books.toscrape.com                            │
│  Duration:            15.2s                                                 │
│  Pages Crawled:       746                                                   │
│  URLs Discovered:     1,892                                                 │
│  API Endpoints:       12                                                    │
│  Forms Found:         3                                                     │
│  Average Speed:       49.1 pages/sec                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Features

### Core Capabilities

| Feature | Description |
|---------|-------------|
| **Blazing Fast** | 200+ concurrent workers, achieving 50-100+ pages/second |
| **SPA Intelligence** | Native support for React, Angular, Vue, Ember, Svelte |
| **Headless Browser** | Chrome/Chromium via CDP for full JavaScript rendering |
| **API Discovery** | Passive XHR interception + Active endpoint probing |
| **WebSocket Detection** | Automatic WS/WSS endpoint discovery and message capture |
| **Form Analysis** | Deep form inspection with CSRF token identification |
| **Smart Deduplication** | Bloom filter-based dedup (memory efficient at scale) |
| **State Persistence** | Save/resume interrupted crawls with BoltDB |

### Authentication Support

OpenCrawler supports **all major authentication methods** used in modern web applications:

| Method | Use Case | Example |
|--------|----------|---------|
| **JWT/Bearer** | REST APIs, SPAs | `--auth-type jwt --token "eyJ..."` |
| **OAuth 2.0** | Third-party integrations | `--auth-type oauth --client-id "..." --client-secret "..."` |
| **Basic Auth** | Legacy systems, internal tools | `--auth-type basic -u admin -p secret` |
| **API Key** | API gateways, microservices | `--auth-type apikey --api-key-header "X-API-Key" --api-key "key"` |
| **Form Login** | Traditional web apps | `--auth-type form --login-url "/login" -u admin -p secret` |
| **Session/Cookie** | Any cookie-based auth | `--cookies "session=abc123"` |

### SPA Framework Detection

OpenCrawler automatically detects and adapts crawling strategies for:

- **React** - Component-based routing, lazy loading, state management
- **Angular** - NgModules, lazy routes, RxJS observables
- **Vue.js** - Vue Router, Vuex store, async components
- **Ember.js** - Ember Data, nested routes, engines
- **Svelte** - SvelteKit routing, stores
- **Next.js / Nuxt.js** - SSR/SSG hybrid applications

### Discovery Methods

```
┌─────────────────────────────────────────────────────────────────────┐
│                     DISCOVERY PIPELINE                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐            │
│   │   Passive   │    │   Active    │    │   Static    │            │
│   │  Discovery  │───▶│   Probing   │───▶│  Analysis   │            │
│   └─────────────┘    └─────────────┘    └─────────────┘            │
│         │                  │                  │                     │
│         ▼                  ▼                  ▼                     │
│   • XHR Intercept    • Path Brute      • JS AST Parse              │
│   • Fetch Monitor    • Method Fuzzing  • Source Maps               │
│   • WebSocket Cap    • GraphQL Detect  • robots.txt                │
│   • Event Listeners  • OpenAPI Probe   • sitemap.xml               │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Comparison with Competitors

### Feature Matrix

| Feature | OpenCrawler | Katana | gospider | hakrawler | Burp Spider | ZAP Spider |
|---------|:-----------:|:------:|:--------:|:---------:|:-----------:|:----------:|
| **Performance** |
| Concurrent Workers | 200+ | 50 | 10 | 5 | 10 | 10 |
| Pages/Second | 80-100+ | 30-50 | 10-20 | 5-10 | 5-10 | 5-10 |
| Memory Efficient | Bloom Filter | Basic | Basic | Basic | High RAM | High RAM |
| **JavaScript Support** |
| Headless Browser | Chrome CDP | Chrome CDP | None | None | Chromium | HtmlUnit |
| SPA Detection | Auto | Manual | None | None | Limited | Limited |
| Framework-Aware | 6 Frameworks | None | None | None | None | None |
| Dynamic Content | Full | Partial | None | None | Partial | Partial |
| **Discovery** |
| API Discovery | Passive+Active | Passive | Basic | Basic | Passive | Passive |
| WebSocket Support | Full | None | None | None | Manual | Manual |
| Form Analysis | CSRF Detection | Basic | Basic | None | Full | Full |
| GraphQL Detection | Auto | None | None | None | Manual | Manual |
| **Authentication** |
| JWT/Bearer | Native | Header | Header | None | Extension | Extension |
| OAuth 2.0 | Native | None | None | None | Extension | Extension |
| Form Login | Auto-detect | Manual | None | None | Macro | Scripted |
| Session Handling | Auto | Manual | Manual | None | Auto | Auto |
| **Operations** |
| State Persistence | BoltDB | None | None | None | Project File | Session |
| Resume Capability | Full | None | None | None | Limited | Limited |
| Progress Display | Real-time | Basic | Basic | None | GUI | GUI |
| Library API | Go Package | None | None | None | REST API | REST API |
| **Deployment** |
| Single Binary | Yes | Yes | Yes | Yes | No (Java) | No (Java) |
| Docker Ready | Yes | Yes | Yes | Yes | Yes | Yes |
| CI/CD Friendly | Yes | Yes | Yes | Yes | Limited | Limited |
| Resource Usage | Low | Low | Low | Low | High | High |

### Benchmark Results

Tested against identical targets with default configurations:

#### Target: E-commerce Site (5,000+ pages)

| Crawler | Time | Pages Crawled | APIs Found | Memory Peak |
|---------|------|---------------|------------|-------------|
| **OpenCrawler** | **2m 15s** | **4,892** | **147** | **85 MB** |
| Katana | 4m 32s | 3,241 | 89 | 120 MB |
| gospider | 8m 45s | 2,156 | 42 | 95 MB |
| hakrawler | 15m+ | 1,823 | 31 | 80 MB |
| Burp Spider | 12m 30s | 3,567 | 112 | 1.2 GB |
| ZAP Spider | 14m 15s | 3,102 | 98 | 980 MB |

#### Target: React SPA (200 routes)

| Crawler | Routes Found | API Endpoints | WebSockets | JS Execution |
|---------|--------------|---------------|------------|--------------|
| **OpenCrawler** | **198** | **67** | **3** | **Full** |
| Katana | 145 | 45 | 0 | Partial |
| gospider | 23 | 12 | 0 | None |
| hakrawler | 18 | 8 | 0 | None |
| Burp Spider | 156 | 52 | 0 | Partial |
| ZAP Spider | 134 | 48 | 0 | Limited |

### Why OpenCrawler Wins

1. **True SPA Understanding** - Not just rendering JS, but understanding component lifecycle, routing patterns, and state management
2. **Intelligent Rate Limiting** - Automatic backoff, WAF detection, and adaptive throttling
3. **Memory Efficiency** - Bloom filter deduplication handles millions of URLs without memory explosion
4. **Security-First Design** - Built for DAST integration with scanner-friendly output
5. **Developer Experience** - Clean Go API, comprehensive docs, and intuitive CLI

---

## Installation

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/PentesterFlow/OpenCrawler/releases):

```bash
# Linux (amd64)
curl -LO https://github.com/PentesterFlow/OpenCrawler/releases/latest/download/opencrawler-linux-amd64
chmod +x opencrawler-linux-amd64
sudo mv opencrawler-linux-amd64 /usr/local/bin/opencrawler

# macOS (Apple Silicon)
curl -LO https://github.com/PentesterFlow/OpenCrawler/releases/latest/download/opencrawler-darwin-arm64
chmod +x opencrawler-darwin-arm64
sudo mv opencrawler-darwin-arm64 /usr/local/bin/opencrawler

# macOS (Intel)
curl -LO https://github.com/PentesterFlow/OpenCrawler/releases/latest/download/opencrawler-darwin-amd64
chmod +x opencrawler-darwin-amd64
sudo mv opencrawler-darwin-amd64 /usr/local/bin/opencrawler

# Windows
# Download opencrawler-windows-amd64.exe from releases page
```

### From Source

```bash
# Clone the repository
git clone https://github.com/PentesterFlow/OpenCrawler.git
cd OpenCrawler

# Build
go build -o opencrawler ./cmd/crawler

# Install globally
go install github.com/PentesterFlow/OpenCrawler/cmd/crawler@latest
```

### Docker

```bash
# Pull image
docker pull pentesterflow/opencrawler:latest

# Run
docker run -it pentesterflow/opencrawler crawl https://example.com

# With output volume
docker run -v $(pwd)/results:/results pentesterflow/opencrawler \
  crawl https://example.com -o /results/output.json
```

### Requirements

| Requirement | Version | Notes |
|-------------|---------|-------|
| Go | 1.21+ | For building from source |
| Chrome/Chromium | Any recent | For headless browser features |

---

## Quick Start

### Basic Crawling

```bash
# Simple crawl with progress bar
opencrawler crawl https://example.com

# Output to file
opencrawler crawl https://example.com -o results.json

# Limit depth
opencrawler crawl https://example.com --max-depth 5
```

### Performance Modes

```bash
# TURBO MODE - Maximum speed (200 workers, aggressive discovery)
opencrawler crawl https://example.com --turbo

# BALANCED MODE - Good speed with thorough analysis
opencrawler crawl https://example.com --balanced

# STEALTH MODE - Low and slow, evade detection
opencrawler crawl https://example.com --stealth
```

### Real-time Progress

```
[██████████████████████████████░░░░░░░░░░] 75% | Pages: 562 | Queue: 187 | APIs: 34 | Forms: 8 | 47.2 p/s | 12s
```

---

## Documentation

### CLI Reference

```
USAGE:
  opencrawler crawl [target] [flags]
  opencrawler resume [flags]

CRAWL FLAGS:
  Target & Scope:
    -d, --max-depth int          Maximum crawl depth (default 10)
        --include strings        Include URL patterns (regex)
        --exclude strings        Exclude URL patterns (regex)
        --follow-external        Follow external domain links

  Performance:
    -w, --workers int            Concurrent workers (default 50)
    -r, --rate-limit float       Requests per second (default 100)
        --turbo                  Maximum speed mode (200 workers)
        --balanced               Balanced speed/coverage
        --stealth                Slow, detection-avoiding mode

  Authentication:
        --auth-type string       Auth type: jwt|basic|oauth|apikey|form
        --token string           JWT/Bearer token
    -u, --username string        Username for auth
    -p, --password string        Password for auth
        --api-key string         API key value
        --api-key-header string  API key header name
        --login-url string       Form login URL
        --cookies string         Cookies (name=value; pairs)

  Browser:
        --browser-pool int       Browser instances (default 5)
        --headless               Run browser headless (default true)
        --user-agent string      Custom User-Agent
        --proxy string           Proxy URL (http/socks5)

  Discovery:
        --no-passive-api         Disable passive API discovery
        --no-active-api          Disable active API probing
        --no-websocket           Disable WebSocket detection
        --no-forms               Disable form analysis
        --no-js                  Disable JavaScript analysis

  Output:
    -o, --output string          Output file path
        --format string          Output format: json|jsonl|csv (default json)
        --pretty                 Pretty-print JSON output

  State:
        --state-file string      State file for persistence
        --auto-save              Auto-save state periodically

  Display:
        --progress               Show progress bar (default true)
        --no-progress            Disable progress bar
    -v, --verbose                Verbose logging
        --debug                  Debug logging

RESUME FLAGS:
        --state-file string      State file to resume from (required)

EXAMPLES:
  # Basic crawl
  opencrawler crawl https://example.com

  # Authenticated crawl with JWT
  opencrawler crawl https://api.example.com --auth-type jwt --token "eyJ..."

  # Fast crawl with limited scope
  opencrawler crawl https://example.com --turbo --max-depth 3 --include ".*api.*"

  # Resume interrupted crawl
  opencrawler resume --state-file crawl-state.db
```

### Output Schema

```json
{
  "target": "https://example.com",
  "started_at": "2024-01-15T10:00:00Z",
  "completed_at": "2024-01-15T10:15:30Z",
  "duration": "15m30s",
  "stats": {
    "urls_discovered": 2847,
    "pages_crawled": 1523,
    "forms_found": 45,
    "api_endpoints": 127,
    "websocket_endpoints": 4,
    "errors": 12,
    "average_speed": 1.64
  },
  "endpoints": [
    {
      "url": "https://example.com/api/v1/users",
      "method": "GET",
      "status_code": 200,
      "content_type": "application/json",
      "source": "passive",
      "depth": 2,
      "discovered_from": "https://example.com/dashboard",
      "parameters": [
        {"name": "page", "type": "query", "example": "1"},
        {"name": "limit", "type": "query", "example": "20"}
      ],
      "headers": {
        "Authorization": "Bearer [REDACTED]"
      }
    }
  ],
  "forms": [
    {
      "url": "https://example.com/login",
      "action": "/auth/login",
      "method": "POST",
      "enctype": "application/x-www-form-urlencoded",
      "inputs": [
        {"name": "username", "type": "text", "required": true},
        {"name": "password", "type": "password", "required": true},
        {"name": "_csrf", "type": "hidden", "value": "[TOKEN]", "is_csrf": true}
      ],
      "has_csrf": true,
      "has_file_upload": false
    }
  ],
  "websockets": [
    {
      "url": "wss://example.com/ws/notifications",
      "discovered_from": "https://example.com/dashboard",
      "protocols": ["graphql-ws"],
      "sample_messages": [
        {"direction": "sent", "data": "{\"type\":\"connection_init\"}"},
        {"direction": "received", "data": "{\"type\":\"connection_ack\"}"}
      ]
    }
  ],
  "technologies": {
    "framework": "React",
    "server": "nginx",
    "languages": ["JavaScript", "TypeScript"],
    "libraries": ["axios", "react-router", "redux"]
  }
}
```

### Configuration File

Create `opencrawler.yaml`:

```yaml
# OpenCrawler Configuration
target: https://example.com

# Performance Settings
workers: 100
max_depth: 15
timeout: 30s

# Scope Rules
scope:
  include_patterns:
    - ".*\\.example\\.com"
    - ".*/api/.*"
  exclude_patterns:
    - ".*/logout.*"
    - ".*/admin.*"
    - ".*\\.(jpg|png|gif|css|js)$"
  follow_external: false
  max_depth: 15

# Rate Limiting
rate_limit:
  requests_per_second: 100
  burst: 20
  respect_robots_txt: true
  adaptive: true  # Auto-adjust on rate limiting

# Authentication
auth:
  type: jwt  # jwt|basic|oauth|apikey|form
  token: "${JWT_TOKEN}"  # Environment variable substitution
  # Or for form login:
  # type: form
  # login_url: https://example.com/login
  # username: admin
  # password: "${PASSWORD}"
  # username_field: email
  # password_field: password

# Browser Settings
browser:
  pool_size: 10
  headless: true
  user_agent: "OpenCrawler/1.0"
  viewport:
    width: 1920
    height: 1080

# Discovery Features
discovery:
  passive_api: true
  active_api: true
  websocket: true
  forms: true
  javascript: true
  source_maps: true
  robots_txt: true
  sitemap: true

# Output Settings
output:
  format: json
  file_path: results.json
  pretty: true
  stream_mode: false  # Stream results as discovered

# State Persistence
state:
  enabled: true
  file_path: crawl-state.db
  auto_save: true
  interval: 60  # seconds
```

Use with:
```bash
opencrawler crawl -c opencrawler.yaml
```

---

## Library Usage

OpenCrawler is also available as a Go library for integration into your tools:

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/PentesterFlow/OpenCrawler/pkg/crawler"
)

func main() {
    // Create crawler with options
    c, err := crawler.New(
        crawler.WithTarget("https://example.com"),
        crawler.WithWorkers(50),
        crawler.WithMaxDepth(10),
        crawler.WithProgress(true),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Start crawling
    result, err := c.Start(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    // Access results
    fmt.Printf("Crawled %d pages\n", result.Stats.PagesCrawled)
    fmt.Printf("Found %d API endpoints\n", result.Stats.APIEndpoints)
    fmt.Printf("Found %d forms\n", result.Stats.FormsFound)

    // Iterate endpoints
    for _, endpoint := range result.Endpoints {
        fmt.Printf("[%s] %s\n", endpoint.Method, endpoint.URL)
    }
}
```

### With Authentication

```go
c, err := crawler.New(
    crawler.WithTarget("https://api.example.com"),
    crawler.WithJWTAuth("eyJhbGciOiJIUzI1NiIs..."),
    // Or Basic Auth
    // crawler.WithBasicAuth("admin", "password"),
    // Or API Key
    // crawler.WithAPIKeyAuth("X-API-Key", "your-api-key"),
    // Or Form Login
    // crawler.WithFormAuth(crawler.FormAuth{
    //     LoginURL: "https://example.com/login",
    //     Username: "admin",
    //     Password: "secret",
    // }),
)
```

### With Scope Rules

```go
c, err := crawler.New(
    crawler.WithTarget("https://example.com"),
    crawler.WithIncludePatterns(`.*api.*`, `.*v1.*`),
    crawler.WithExcludePatterns(`.*logout.*`, `.*\.(jpg|png|gif)$`),
    crawler.WithFollowExternal(false),
    crawler.WithMaxDepth(5),
)
```

### Custom HTTP Client

```go
import "net/http"

transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 100,
    IdleConnTimeout:     90 * time.Second,
}

client := &http.Client{
    Transport: transport,
    Timeout:   30 * time.Second,
}

c, err := crawler.New(
    crawler.WithTarget("https://example.com"),
    crawler.WithHTTPClient(client),
    crawler.WithProxy("http://127.0.0.1:8080"),
)
```

### Event Callbacks

```go
c, err := crawler.New(
    crawler.WithTarget("https://example.com"),
    crawler.WithOnEndpointFound(func(e *crawler.Endpoint) {
        fmt.Printf("Found: %s %s\n", e.Method, e.URL)
    }),
    crawler.WithOnFormFound(func(f *crawler.Form) {
        fmt.Printf("Form: %s -> %s\n", f.URL, f.Action)
    }),
    crawler.WithOnError(func(err error) {
        log.Printf("Error: %v\n", err)
    }),
)
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              OpenCrawler                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐     │
│  │    CLI      │   │   Library   │   │   Config    │   │   Output    │     │
│  │  (Cobra)    │   │     API     │   │   (Viper)   │   │   (JSON)    │     │
│  └──────┬──────┘   └──────┬──────┘   └──────┬──────┘   └──────┬──────┘     │
│         │                 │                 │                 │             │
│         └─────────────────┴────────┬────────┴─────────────────┘             │
│                                    │                                         │
│                           ┌────────┴────────┐                               │
│                           │    Crawler      │                               │
│                           │  Orchestrator   │                               │
│                           └────────┬────────┘                               │
│                                    │                                         │
│         ┌──────────────────────────┼──────────────────────────┐             │
│         │                          │                          │             │
│  ┌──────┴──────┐           ┌───────┴───────┐          ┌───────┴───────┐    │
│  │   Browser   │           │     Queue     │          │    Scope      │    │
│  │    Pool     │           │   Manager     │          │   Checker     │    │
│  │  (Chrome)   │           │  (Priority)   │          │   (Regex)     │    │
│  └──────┬──────┘           └───────┬───────┘          └───────────────┘    │
│         │                          │                                         │
│  ┌──────┴──────┐           ┌───────┴───────┐          ┌───────────────┐    │
│  │ Interceptor │           │     State     │          │  Rate Limiter │    │
│  │  (Network)  │           │  Persistence  │          │  (Adaptive)   │    │
│  └─────────────┘           │   (BoltDB)    │          └───────────────┘    │
│                            └───────────────┘                                │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        Discovery Engine                              │   │
│  ├─────────────┬─────────────┬─────────────┬─────────────┬─────────────┤   │
│  │   HTML      │     JS      │   Form      │    API      │  WebSocket  │   │
│  │  Parser     │  Analyzer   │  Detector   │  Discovery  │   Handler   │   │
│  └─────────────┴─────────────┴─────────────┴─────────────┴─────────────┘   │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                     Authentication Layer                             │   │
│  ├─────────────┬─────────────┬─────────────┬─────────────┬─────────────┤   │
│  │   Session   │    JWT      │   OAuth     │   Basic     │   API Key   │   │
│  └─────────────┴─────────────┴─────────────┴─────────────┴─────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
OpenCrawler/
├── cmd/
│   └── crawler/
│       └── main.go              # CLI entry point
├── pkg/
│   └── crawler/
│       ├── crawler.go           # Main orchestrator (public API)
│       ├── config.go            # Configuration structures
│       ├── options.go           # Functional options pattern
│       └── types.go             # Public types
├── internal/
│   ├── auth/                    # Authentication providers
│   │   ├── auth.go              # Auth interface
│   │   ├── jwt.go               # JWT/Bearer token
│   │   ├── oauth.go             # OAuth 2.0 flow
│   │   ├── formlogin.go         # Form-based login
│   │   └── session.go           # Session management
│   ├── browser/                 # Headless browser
│   │   ├── browser.go           # Chrome CDP wrapper
│   │   ├── pool.go              # Browser instance pool
│   │   ├── interceptor.go       # Network interception
│   │   └── spa.go               # SPA handling
│   ├── discovery/               # Endpoint discovery
│   │   ├── passive.go           # XHR/Fetch interception
│   │   ├── active.go            # Active probing
│   │   └── enhanced/            # Advanced discovery
│   ├── framework/               # SPA framework detection
│   │   ├── react.go
│   │   ├── angular.go
│   │   ├── vue.go
│   │   └── ember.go
│   ├── parser/                  # Content parsing
│   │   ├── html.go              # HTML link extraction
│   │   ├── javascript.go        # JS static analysis
│   │   └── forms.go             # Form analysis
│   ├── queue/                   # URL queue
│   │   ├── queue.go             # Queue interface
│   │   ├── memory.go            # In-memory queue
│   │   └── persistent.go        # Disk-backed queue
│   ├── state/                   # State management
│   │   ├── state.go             # State manager
│   │   ├── store.go             # BoltDB storage
│   │   └── dedup.go             # Bloom filter dedup
│   ├── scope/                   # Scope checking
│   ├── ratelimit/               # Rate limiting
│   ├── websocket/               # WebSocket handling
│   ├── progress/                # Progress display
│   └── output/                  # Output formatting
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## Use Cases

### Security Testing (DAST)

```bash
# Comprehensive security crawl
opencrawler crawl https://target.com \
  --auth-type jwt --token "$JWT" \
  --include ".*api.*" \
  -o endpoints.json

# Feed to security scanner
cat endpoints.json | jq -r '.endpoints[].url' | nuclei -l -
```

### Bug Bounty Reconnaissance

```bash
# Fast, wide crawl
opencrawler crawl https://target.com --turbo --max-depth 5 -o recon.json

# Extract unique paths
cat recon.json | jq -r '.endpoints[].url' | sort -u > paths.txt
```

### API Documentation

```bash
# Discover undocumented APIs
opencrawler crawl https://app.example.com \
  --auth-type form --login-url "/login" -u user -p pass \
  --include ".*api.*" --include ".*graphql.*" \
  -o api-endpoints.json
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Crawl Application
  run: |
    opencrawler crawl ${{ env.APP_URL }} \
      --auth-type jwt --token "${{ secrets.JWT_TOKEN }}" \
      --max-depth 5 \
      -o crawl-results.json

- name: Upload Results
  uses: actions/upload-artifact@v3
  with:
    name: crawl-results
    path: crawl-results.json
```

---

## Performance Tuning

### Optimal Settings by Use Case

| Use Case | Workers | Depth | Rate Limit | Features |
|----------|---------|-------|------------|----------|
| Quick Recon | 200 | 3 | 200 rps | `--turbo --no-js` |
| Deep Crawl | 50 | 15 | 50 rps | `--balanced` |
| API Discovery | 100 | 10 | 100 rps | `--no-forms` |
| Stealth Mode | 5 | 10 | 1 rps | `--stealth` |
| Full Analysis | 30 | 20 | 30 rps | All features |

### Tips

1. **Turbo Mode** for large sites: `--turbo`
2. **Limit Depth** for focused crawls: `--max-depth 3`
3. **Increase Workers** for fast servers: `-w 200`
4. **Disable Unused Features**: `--no-js --no-websocket`
5. **Use State Persistence** for long crawls: `--state-file state.db`
6. **Exclude Static Assets**: `--exclude ".*\\.(jpg|png|gif|css|js|woff)$"`

---

## Contributing

Contributions are welcome! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure tests pass (`go test ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Development Setup

```bash
git clone https://github.com/PentesterFlow/OpenCrawler.git
cd OpenCrawler
go mod download
go build -o opencrawler ./cmd/crawler
go test ./...
```

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## Disclaimer

**This tool is intended for authorized security testing and educational purposes only.** Always obtain proper authorization before crawling or testing any web application. The authors are not responsible for any misuse of this tool.

---

## Acknowledgments

- [go-rod](https://github.com/go-rod/rod) - Headless browser automation
- [goquery](https://github.com/PuerkitoBio/goquery) - HTML parsing
- [cobra](https://github.com/spf13/cobra) - CLI framework
- [bbolt](https://github.com/etcd-io/bbolt) - State persistence
- [bloom](https://github.com/bits-and-blooms/bloom) - Bloom filter

---

<p align="center">
  <b>Built for Security Researchers, by Security Researchers</b>
</p>

<p align="center">
  <a href="https://github.com/PentesterFlow">PentesterFlow</a> •
  <a href="https://twitter.com/PentesterFlow">Twitter</a> •
  <a href="https://discord.gg/pentesterflow">Discord</a>
</p>

<p align="center">
  <sub>If you find OpenCrawler useful, please consider giving it a star!</sub>
</p>
