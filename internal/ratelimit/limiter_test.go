package ratelimit

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Limiter Tests
// =============================================================================

func TestNewLimiter(t *testing.T) {
	l := NewLimiter(10.0, 5)

	if l == nil {
		t.Fatal("NewLimiter() returned nil")
	}
	if l.limiter == nil {
		t.Error("limiter is nil")
	}
	if l.perDomain == nil {
		t.Error("perDomain map is nil")
	}
	if l.lastRequest == nil {
		t.Error("lastRequest map is nil")
	}
	if l.robots == nil {
		t.Error("robots manager is nil")
	}
	if l.defaultRate != 10.0 {
		t.Errorf("defaultRate = %v, want 10.0", l.defaultRate)
	}
	if l.defaultBurst != 5 {
		t.Errorf("defaultBurst = %d, want 5", l.defaultBurst)
	}
}

func TestLimiter_Allow(t *testing.T) {
	l := NewLimiter(1000, 10) // High rate for testing

	// Should allow first request
	if !l.Allow() {
		t.Error("Allow() should return true for first request")
	}
}

func TestLimiter_Allow_Burst(t *testing.T) {
	l := NewLimiter(1, 3) // 1 req/sec with burst of 3

	// First 3 requests should be allowed (burst)
	for i := 0; i < 3; i++ {
		if !l.Allow() {
			t.Errorf("Allow() should return true for burst request %d", i+1)
		}
	}

	// Fourth request should be denied (burst exhausted)
	if l.Allow() {
		t.Error("Allow() should return false after burst exhausted")
	}
}

func TestLimiter_Wait(t *testing.T) {
	l := NewLimiter(1000, 10)
	ctx := context.Background()

	err := l.Wait(ctx)
	if err != nil {
		t.Errorf("Wait() error = %v", err)
	}
}

func TestLimiter_Wait_ContextCancelled(t *testing.T) {
	l := NewLimiter(0.1, 1) // Very slow rate
	l.Allow()               // Exhaust burst

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := l.Wait(ctx)
	if err == nil {
		t.Error("Wait() should return error for cancelled context")
	}
}

func TestLimiter_WaitDomain(t *testing.T) {
	l := NewLimiter(1000, 10)
	ctx := context.Background()

	err := l.WaitDomain(ctx, "example.com")
	if err != nil {
		t.Errorf("WaitDomain() error = %v", err)
	}

	// Should create a domain-specific limiter
	l.mu.RLock()
	_, exists := l.perDomain["example.com"]
	l.mu.RUnlock()

	if !exists {
		t.Error("WaitDomain should create per-domain limiter")
	}
}

func TestLimiter_WaitDomain_WithDelay(t *testing.T) {
	l := NewLimiter(1000, 10)
	l.SetDomainDelay(50 * time.Millisecond)
	ctx := context.Background()

	// First request
	start := time.Now()
	err := l.WaitDomain(ctx, "example.com")
	if err != nil {
		t.Errorf("WaitDomain() error = %v", err)
	}

	// Second request should be delayed
	err = l.WaitDomain(ctx, "example.com")
	if err != nil {
		t.Errorf("WaitDomain() error = %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 50*time.Millisecond {
		t.Errorf("Domain delay not enforced: elapsed = %v", elapsed)
	}
}

func TestLimiter_SetDomainRate(t *testing.T) {
	l := NewLimiter(10.0, 5)

	l.SetDomainRate("slow.com", 1.0, 1)

	l.mu.RLock()
	domainLimiter, exists := l.perDomain["slow.com"]
	l.mu.RUnlock()

	if !exists {
		t.Error("SetDomainRate should create domain limiter")
	}
	if domainLimiter == nil {
		t.Error("Domain limiter should not be nil")
	}
}

func TestLimiter_SetDomainDelay(t *testing.T) {
	l := NewLimiter(10.0, 5)

	l.SetDomainDelay(100 * time.Millisecond)

	if l.domainDelay != 100*time.Millisecond {
		t.Errorf("domainDelay = %v, want 100ms", l.domainDelay)
	}
}

func TestLimiter_AllowDomain(t *testing.T) {
	l := NewLimiter(1000, 10)

	// First request to new domain should be allowed
	if !l.AllowDomain("example.com") {
		t.Error("AllowDomain should return true for first request")
	}
}

func TestLimiter_AllowDomain_WithCustomRate(t *testing.T) {
	l := NewLimiter(1000, 10)
	l.SetDomainRate("slow.com", 1.0, 1)

	// First request allowed (burst)
	if !l.AllowDomain("slow.com") {
		t.Error("AllowDomain should return true for burst")
	}

	// Second request denied (burst exhausted, rate limited)
	if l.AllowDomain("slow.com") {
		t.Error("AllowDomain should return false after burst exhausted")
	}
}

func TestLimiter_Reserve(t *testing.T) {
	l := NewLimiter(10.0, 5)

	reservation := l.Reserve()
	if reservation == nil {
		t.Error("Reserve() returned nil")
	}
}

func TestLimiter_SetRate(t *testing.T) {
	l := NewLimiter(10.0, 5)

	l.SetRate(20.0, 10)

	if l.defaultRate != 20.0 {
		t.Errorf("defaultRate = %v, want 20.0", l.defaultRate)
	}
	if l.defaultBurst != 10 {
		t.Errorf("defaultBurst = %d, want 10", l.defaultBurst)
	}
}

func TestLimiter_GetRobots(t *testing.T) {
	l := NewLimiter(10.0, 5)

	robots := l.GetRobots()
	if robots == nil {
		t.Error("GetRobots() returned nil")
	}
}

func TestLimiter_IsAllowed(t *testing.T) {
	l := NewLimiter(10.0, 5)
	ctx := context.Background()

	// Without robots check
	if !l.IsAllowed(ctx, "example.com", "/test", "TestBot", false) {
		t.Error("IsAllowed should return true without robots check")
	}

	// With robots check (no robots.txt fetched yet, default allow)
	if !l.IsAllowed(ctx, "example.com", "/test", "TestBot", true) {
		t.Error("IsAllowed should return true when no robots.txt")
	}
}

func TestLimiter_GetCrawlDelay(t *testing.T) {
	l := NewLimiter(10.0, 5)

	// No robots.txt fetched, should return 0
	delay := l.GetCrawlDelay("example.com", "TestBot")
	if delay != 0 {
		t.Errorf("GetCrawlDelay = %v, want 0", delay)
	}
}

func TestLimiter_Stats(t *testing.T) {
	l := NewLimiter(10.0, 5)
	l.SetDomainDelay(100 * time.Millisecond)
	l.SetDomainRate("example.com", 5.0, 2)

	stats := l.Stats()

	if stats.DomainCount != 1 {
		t.Errorf("DomainCount = %d, want 1", stats.DomainCount)
	}
	if stats.DefaultRate != 10.0 {
		t.Errorf("DefaultRate = %v, want 10.0", stats.DefaultRate)
	}
	if stats.DefaultBurst != 5 {
		t.Errorf("DefaultBurst = %d, want 5", stats.DefaultBurst)
	}
	if stats.DomainDelay != 100*time.Millisecond {
		t.Errorf("DomainDelay = %v, want 100ms", stats.DomainDelay)
	}
}

func TestLimiter_Concurrent(t *testing.T) {
	l := NewLimiter(1000, 100)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(domain string) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				l.WaitDomain(ctx, domain)
				l.AllowDomain(domain)
			}
		}("domain" + string(rune('0'+i)))
	}
	wg.Wait()

	// Should have 10 domain limiters
	stats := l.Stats()
	if stats.DomainCount != 10 {
		t.Errorf("DomainCount = %d, want 10", stats.DomainCount)
	}
}

// =============================================================================
// AdaptiveRateLimiter Tests
// =============================================================================

func TestNewAdaptiveRateLimiter(t *testing.T) {
	a := NewAdaptiveRateLimiter(1.0, 100.0, 10)

	if a == nil {
		t.Fatal("NewAdaptiveRateLimiter() returned nil")
	}
	if a.Limiter == nil {
		t.Error("Embedded Limiter is nil")
	}
	if a.minRate != 1.0 {
		t.Errorf("minRate = %v, want 1.0", a.minRate)
	}
	if a.maxRate != 100.0 {
		t.Errorf("maxRate = %v, want 100.0", a.maxRate)
	}
	if a.currentRate != 100.0 {
		t.Errorf("currentRate = %v, want 100.0 (starts at max)", a.currentRate)
	}
	if a.windowSize != 100 {
		t.Errorf("windowSize = %d, want 100", a.windowSize)
	}
}

func TestAdaptiveRateLimiter_RecordSuccess(t *testing.T) {
	a := NewAdaptiveRateLimiter(1.0, 100.0, 10)

	a.RecordSuccess()

	a.mu.Lock()
	if a.successCount != 1 {
		t.Errorf("successCount = %d, want 1", a.successCount)
	}
	a.mu.Unlock()
}

func TestAdaptiveRateLimiter_RecordError(t *testing.T) {
	a := NewAdaptiveRateLimiter(1.0, 100.0, 10)

	a.RecordError()

	a.mu.Lock()
	if a.errorCount != 1 {
		t.Errorf("errorCount = %d, want 1", a.errorCount)
	}
	a.mu.Unlock()
}

func TestAdaptiveRateLimiter_CurrentRate(t *testing.T) {
	a := NewAdaptiveRateLimiter(1.0, 100.0, 10)

	rate := a.CurrentRate()
	if rate != 100.0 {
		t.Errorf("CurrentRate() = %v, want 100.0", rate)
	}
}

func TestAdaptiveRateLimiter_SlowDown(t *testing.T) {
	a := NewAdaptiveRateLimiter(1.0, 100.0, 10)
	a.windowSize = 10 // Small window for testing

	// Record many errors
	for i := 0; i < 5; i++ {
		a.RecordSuccess()
	}
	for i := 0; i < 5; i++ {
		a.RecordError()
	}

	// Should slow down due to 50% error rate
	rate := a.CurrentRate()
	if rate >= 100.0 {
		t.Errorf("CurrentRate() = %v, should be less than 100.0 after errors", rate)
	}
}

func TestAdaptiveRateLimiter_SpeedUp(t *testing.T) {
	a := NewAdaptiveRateLimiter(1.0, 100.0, 10)
	a.windowSize = 10 // Small window for testing
	a.currentRate = 50.0
	a.SetRate(50.0, 10)

	// Record all successes
	for i := 0; i < 10; i++ {
		a.RecordSuccess()
	}

	// Should speed up due to 0% error rate
	rate := a.CurrentRate()
	if rate <= 50.0 {
		t.Errorf("CurrentRate() = %v, should be greater than 50.0 after successes", rate)
	}
}

func TestAdaptiveRateLimiter_MinRate(t *testing.T) {
	a := NewAdaptiveRateLimiter(10.0, 100.0, 10)
	a.windowSize = 10
	a.currentRate = 11.0 // Just above min
	a.SetRate(11.0, 10)

	// Record all errors
	for i := 0; i < 10; i++ {
		a.RecordError()
	}

	// Should not go below minRate
	rate := a.CurrentRate()
	if rate < 10.0 {
		t.Errorf("CurrentRate() = %v, should not go below minRate 10.0", rate)
	}
}

func TestAdaptiveRateLimiter_MaxRate(t *testing.T) {
	a := NewAdaptiveRateLimiter(1.0, 100.0, 10)
	a.windowSize = 10

	// Start at max and record all successes
	for i := 0; i < 10; i++ {
		a.RecordSuccess()
	}

	// Should not exceed maxRate
	rate := a.CurrentRate()
	if rate > 100.0 {
		t.Errorf("CurrentRate() = %v, should not exceed maxRate 100.0", rate)
	}
}

// =============================================================================
// RobotsManager Tests
// =============================================================================

func TestNewRobotsManager(t *testing.T) {
	m := NewRobotsManager()

	if m == nil {
		t.Fatal("NewRobotsManager() returned nil")
	}
	if m.rules == nil {
		t.Error("rules map is nil")
	}
	if m.client == nil {
		t.Error("client is nil")
	}
	if m.cache != 1*time.Hour {
		t.Errorf("cache = %v, want 1h", m.cache)
	}
}

func TestRobotsManager_IsAllowed_NoRules(t *testing.T) {
	m := NewRobotsManager()

	// No rules fetched yet, should allow by default
	allowed := m.IsAllowed("example.com", "/test", "TestBot")
	if !allowed {
		t.Error("IsAllowed should return true when no rules fetched")
	}
}

func TestRobotsManager_GetCrawlDelay_NoRules(t *testing.T) {
	m := NewRobotsManager()

	delay := m.GetCrawlDelay("example.com", "TestBot")
	if delay != 0 {
		t.Errorf("GetCrawlDelay = %v, want 0 when no rules", delay)
	}
}

func TestRobotsManager_GetSitemaps_NoRules(t *testing.T) {
	m := NewRobotsManager()

	sitemaps := m.GetSitemaps("example.com")
	if sitemaps != nil {
		t.Error("GetSitemaps should return nil when no rules")
	}
}

// =============================================================================
// ParseRobots Tests
// =============================================================================

func TestParseRobots_Basic(t *testing.T) {
	content := `
User-agent: *
Disallow: /admin/
Disallow: /private/
Allow: /public/
Crawl-delay: 2
Sitemap: https://example.com/sitemap.xml
`
	rules, err := ParseRobots(strings.NewReader(content), "TestBot")
	if err != nil {
		t.Fatalf("ParseRobots() error = %v", err)
	}

	if len(rules.Disallow) != 2 {
		t.Errorf("Disallow count = %d, want 2", len(rules.Disallow))
	}
	if len(rules.Allow) != 1 {
		t.Errorf("Allow count = %d, want 1", len(rules.Allow))
	}
	if rules.CrawlDelay != 2*time.Second {
		t.Errorf("CrawlDelay = %v, want 2s", rules.CrawlDelay)
	}
	if len(rules.Sitemaps) != 1 {
		t.Errorf("Sitemaps count = %d, want 1", len(rules.Sitemaps))
	}
}

func TestParseRobots_SpecificUserAgent(t *testing.T) {
	content := `
User-agent: Googlebot
Disallow: /google-only/

User-agent: TestBot
Disallow: /test-only/

User-agent: *
Disallow: /all/
`
	rules, err := ParseRobots(strings.NewReader(content), "TestBot")
	if err != nil {
		t.Fatalf("ParseRobots() error = %v", err)
	}

	// Should match TestBot rules
	if !rules.IsAllowed("/google-only/page") {
		t.Error("Should allow /google-only/ for TestBot")
	}
	if rules.IsAllowed("/test-only/page") {
		t.Error("Should disallow /test-only/ for TestBot")
	}
}

func TestParseRobots_Comments(t *testing.T) {
	content := `
# This is a comment
User-agent: *
# Another comment
Disallow: /admin/
`
	rules, err := ParseRobots(strings.NewReader(content), "TestBot")
	if err != nil {
		t.Fatalf("ParseRobots() error = %v", err)
	}

	if len(rules.Disallow) != 1 {
		t.Errorf("Disallow count = %d, want 1", len(rules.Disallow))
	}
}

func TestParseRobots_EmptyLines(t *testing.T) {
	content := `
User-agent: *

Disallow: /admin/

Allow: /public/
`
	rules, err := ParseRobots(strings.NewReader(content), "TestBot")
	if err != nil {
		t.Fatalf("ParseRobots() error = %v", err)
	}

	if len(rules.Disallow) != 1 {
		t.Errorf("Disallow count = %d, want 1", len(rules.Disallow))
	}
	if len(rules.Allow) != 1 {
		t.Errorf("Allow count = %d, want 1", len(rules.Allow))
	}
}

func TestParseRobots_Wildcard(t *testing.T) {
	content := `
User-agent: *
Disallow: /*.pdf$
Disallow: /private/*
`
	rules, err := ParseRobots(strings.NewReader(content), "TestBot")
	if err != nil {
		t.Fatalf("ParseRobots() error = %v", err)
	}

	// Test PDF pattern
	if rules.IsAllowed("/document.pdf") {
		t.Error("Should disallow .pdf files")
	}
	if !rules.IsAllowed("/document.pdf.bak") {
		t.Error("Should allow .pdf.bak (not ending in .pdf)")
	}

	// Test private/* pattern
	if rules.IsAllowed("/private/secret") {
		t.Error("Should disallow /private/*")
	}
}

func TestParseRobots_MultipleSitemaps(t *testing.T) {
	content := `
User-agent: *
Disallow:

Sitemap: https://example.com/sitemap1.xml
Sitemap: https://example.com/sitemap2.xml
Sitemap: https://example.com/sitemap3.xml
`
	rules, err := ParseRobots(strings.NewReader(content), "TestBot")
	if err != nil {
		t.Fatalf("ParseRobots() error = %v", err)
	}

	if len(rules.Sitemaps) != 3 {
		t.Errorf("Sitemaps count = %d, want 3", len(rules.Sitemaps))
	}
}

func TestParseRobots_FloatCrawlDelay(t *testing.T) {
	content := `
User-agent: *
Crawl-delay: 0.5
`
	rules, err := ParseRobots(strings.NewReader(content), "TestBot")
	if err != nil {
		t.Fatalf("ParseRobots() error = %v", err)
	}

	expected := 500 * time.Millisecond
	if rules.CrawlDelay != expected {
		t.Errorf("CrawlDelay = %v, want %v", rules.CrawlDelay, expected)
	}
}

// =============================================================================
// RobotsRules Tests
// =============================================================================

func TestRobotsRules_IsAllowed_AllowTakesPriority(t *testing.T) {
	content := `
User-agent: *
Disallow: /admin/
Allow: /admin/public/
`
	rules, _ := ParseRobots(strings.NewReader(content), "TestBot")

	// Allow should take priority
	if !rules.IsAllowed("/admin/public/page") {
		t.Error("Allow should take priority over Disallow")
	}
	if rules.IsAllowed("/admin/secret") {
		t.Error("Should disallow /admin/secret")
	}
}

func TestRobotsRules_IsAllowed_DefaultAllow(t *testing.T) {
	rules := &RobotsRules{}

	if !rules.IsAllowed("/anything") {
		t.Error("Should allow by default")
	}
}

// =============================================================================
// pathToRegexp Tests
// =============================================================================

func TestPathToRegexp(t *testing.T) {
	tests := []struct {
		path     string
		input    string
		expected bool
	}{
		{"/admin/", "/admin/page", true},
		{"/admin/", "/public/admin", false},
		{"/*.pdf$", "/doc.pdf", true},
		{"/*.pdf$", "/doc.pdf.bak", false},
		{"/private/*", "/private/secret", true},
		{"/private/*", "/private/", true},
		{"/test", "/test", true},
		{"/test", "/test123", true}, // Prefix match
		{"/test", "/other", false},
	}

	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.input, func(t *testing.T) {
			pattern := pathToRegexp(tt.path)
			rules := &RobotsRules{}
			rules.Disallow = append(rules.Disallow, mustCompile(pattern))

			allowed := rules.IsAllowed(tt.input)
			if allowed == tt.expected {
				t.Errorf("IsAllowed(%s) with pattern %s = %v, want %v",
					tt.input, tt.path, allowed, !tt.expected)
			}
		})
	}
}

func mustCompile(pattern string) *regexp.Regexp {
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	return re
}
