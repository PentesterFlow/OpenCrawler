package parser

import (
	"testing"
)

// =============================================================================
// HTMLParser Tests
// =============================================================================

func TestNewHTMLParser(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{
			name:    "valid URL",
			baseURL: "https://example.com",
			wantErr: false,
		},
		{
			name:    "URL with path",
			baseURL: "https://example.com/path/to/page",
			wantErr: false,
		},
		{
			name:    "invalid URL",
			baseURL: "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewHTMLParser(tt.baseURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHTMLParser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && p == nil {
				t.Error("NewHTMLParser() returned nil parser")
			}
		})
	}
}

func TestHTMLParser_Parse_Links(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<html>
		<body>
			<a href="/page1">Page 1</a>
			<a href="https://example.com/page2">Page 2</a>
			<a href="page3">Page 3</a>
			<a href="/page4" rel="nofollow">No Follow</a>
			<a href="/page5" target="_blank">New Tab</a>
		</body>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Links) != 5 {
		t.Errorf("len(Links) = %d, want 5", len(result.Links))
	}

	// Check first link
	found := false
	for _, link := range result.Links {
		if link.URL == "https://example.com/page1" && link.Text == "Page 1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected link /page1 not found")
	}

	// Check nofollow
	for _, link := range result.Links {
		if link.URL == "https://example.com/page4" && !link.NoFollow {
			t.Error("expected nofollow to be true")
		}
	}
}

func TestHTMLParser_Parse_AngularLinks(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<html>
		<body>
			<a ng-href="/angular-page">Angular Page</a>
			<a ng-href="{{dynamicUrl}}">Dynamic</a>
			<a ui-sref="users.list">Users</a>
			<a ui-sref="users.detail({id: 1})">User Detail</a>
			<a routerlink="/react-page">React Page</a>
		</body>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have ng-href, ui-sref links
	// ng-href with {{}} should be skipped
	// Note: routerLink uses camelCase attribute which goquery handles case-insensitively
	if len(result.Links) < 3 {
		t.Errorf("len(Links) = %d, want at least 3", len(result.Links))
	}
}

func TestHTMLParser_Parse_Forms(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<html>
		<body>
			<form action="/login" method="POST">
				<input type="text" name="username" required placeholder="Username">
				<input type="password" name="password" required>
				<input type="hidden" name="csrf" value="token123">
				<button type="submit">Login</button>
			</form>
		</body>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Forms) != 1 {
		t.Fatalf("len(Forms) = %d, want 1", len(result.Forms))
	}

	form := result.Forms[0]
	if form.Action != "https://example.com/login" {
		t.Errorf("Action = %q, want %q", form.Action, "https://example.com/login")
	}
	if form.Method != "POST" {
		t.Errorf("Method = %q, want POST", form.Method)
	}
	if len(form.Inputs) != 3 {
		t.Errorf("len(Inputs) = %d, want 3", len(form.Inputs))
	}
}

func TestHTMLParser_Parse_FormDefaults(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com/page")

	html := `
		<html>
		<body>
			<form>
				<input name="q">
				<input type="submit" value="Search">
			</form>
		</body>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	form := result.Forms[0]
	if form.Method != "GET" {
		t.Errorf("default Method = %q, want GET", form.Method)
	}
	if form.Enctype != "application/x-www-form-urlencoded" {
		t.Errorf("default Enctype = %q", form.Enctype)
	}
	// Action should default to current page
	if form.Action != "https://example.com/page" {
		t.Errorf("default Action = %q", form.Action)
	}
}

func TestHTMLParser_Parse_Scripts(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<html>
		<head>
			<script src="/js/app.js"></script>
			<script src="https://cdn.example.com/lib.js"></script>
		</head>
		<body>
			<script>console.log("inline")</script>
		</body>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have 2 scripts (src only, not inline)
	if len(result.Scripts) != 2 {
		t.Errorf("len(Scripts) = %d, want 2", len(result.Scripts))
	}
}

func TestHTMLParser_Parse_Stylesheets(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<html>
		<head>
			<link rel="stylesheet" href="/css/style.css">
			<link type="text/css" href="/css/theme.css">
			<link rel="icon" href="/favicon.ico">
		</head>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Stylesheets) != 2 {
		t.Errorf("len(Stylesheets) = %d, want 2", len(result.Stylesheets))
	}
}

func TestHTMLParser_Parse_Images(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<html>
		<body>
			<img src="/images/logo.png">
			<img src="https://example.com/images/banner.jpg">
		</body>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Images) != 2 {
		t.Errorf("len(Images) = %d, want 2", len(result.Images))
	}
}

func TestHTMLParser_Parse_Iframes(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<html>
		<body>
			<iframe src="/embed/video"></iframe>
			<iframe src="https://youtube.com/embed/abc"></iframe>
		</body>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(result.Iframes) != 2 {
		t.Errorf("len(Iframes) = %d, want 2", len(result.Iframes))
	}
}

func TestHTMLParser_Parse_Meta(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<html>
		<head>
			<meta name="description" content="Test page">
			<meta property="og:title" content="OG Title">
			<meta charset="utf-8">
		</head>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if result.Meta["description"] != "Test page" {
		t.Errorf("Meta[description] = %q", result.Meta["description"])
	}
	if result.Meta["og:title"] != "OG Title" {
		t.Errorf("Meta[og:title] = %q", result.Meta["og:title"])
	}
}

func TestHTMLParser_Parse_SkipSpecialURLs(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<html>
		<body>
			<a href="javascript:void(0)">JS Link</a>
			<a href="mailto:test@example.com">Email</a>
			<a href="tel:+1234567890">Phone</a>
			<a href="data:text/html,<h1>Test</h1>">Data</a>
			<a href="#section">Anchor</a>
			<a href="#/spa-route">SPA Route</a>
			<a href="#!/hash-bang">Hash Bang</a>
		</body>
		</html>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should only have SPA routes (#/ and #!/), not js/mailto/tel/data/#anchor
	if len(result.Links) != 2 {
		t.Errorf("len(Links) = %d, want 2 (SPA routes only)", len(result.Links))
	}
}

func TestHTMLParser_Parse_InputTypes(t *testing.T) {
	p, _ := NewHTMLParser("https://example.com")

	html := `
		<form action="/form" method="POST">
			<input type="text" name="text_field">
			<input name="default_type">
			<textarea name="message">Initial text</textarea>
			<select name="country">
				<option value="us">US</option>
				<option value="uk">UK</option>
			</select>
			<input type="file" name="upload" accept="image/*" multiple>
			<input type="checkbox" name="agree" required>
			<input type="hidden" name="token" value="abc123">
		</form>
	`

	result, err := p.Parse(html)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	form := result.Forms[0]
	inputTypes := make(map[string]string)
	for _, input := range form.Inputs {
		inputTypes[input.Name] = input.Type
	}

	// Default type should be "text"
	if inputTypes["default_type"] != "text" {
		t.Errorf("default input type = %q, want text", inputTypes["default_type"])
	}

	if inputTypes["message"] != "textarea" {
		t.Errorf("textarea type = %q", inputTypes["message"])
	}

	if inputTypes["country"] != "select" {
		t.Errorf("select type = %q", inputTypes["country"])
	}
}

func TestExtractURLsFromText(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			name: "multiple URLs",
			text: "Check out https://example.com and http://test.com for more",
			want: 2,
		},
		{
			name: "URL in quotes",
			text: `url: "https://api.example.com/v1/users"`,
			want: 1,
		},
		{
			name: "no URLs",
			text: "This is plain text without URLs",
			want: 0,
		},
		{
			name: "URL with path and query",
			text: "Visit https://example.com/path?q=test&page=1 today",
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := ExtractURLsFromText(tt.text)
			if len(urls) != tt.want {
				t.Errorf("len(urls) = %d, want %d", len(urls), tt.want)
			}
		})
	}
}

// =============================================================================
// FormAnalyzer Tests
// =============================================================================

func TestNewFormAnalyzer(t *testing.T) {
	fa := NewFormAnalyzer()
	if fa == nil {
		t.Fatal("NewFormAnalyzer() returned nil")
	}
}

func TestFormAnalyzer_Analyze_LoginForm(t *testing.T) {
	fa := NewFormAnalyzer()

	form := FormInfo{
		Action:  "/login",
		Method:  "POST",
		Enctype: "application/x-www-form-urlencoded",
		Inputs: []InputInfo{
			{Name: "username", Type: "text"},
			{Name: "password", Type: "password"},
			{Name: "csrf_token", Type: "hidden", Value: "abc123"},
		},
	}

	result := fa.Analyze(form, "https://example.com/login")

	if result.FormType != FormTypeLogin {
		t.Errorf("FormType = %q, want %q", result.FormType, FormTypeLogin)
	}
	if !result.IsLogin {
		t.Error("IsLogin should be true")
	}
	if !result.HasCSRF {
		t.Error("HasCSRF should be true")
	}
	if result.CSRFField != "csrf_token" {
		t.Errorf("CSRFField = %q", result.CSRFField)
	}
}

func TestFormAnalyzer_Analyze_SignupForm(t *testing.T) {
	fa := NewFormAnalyzer()

	form := FormInfo{
		Action:  "/signup",
		Method:  "POST",
		Inputs: []InputInfo{
			{Name: "email", Type: "email"},
			{Name: "username", Type: "text"},
			{Name: "password", Type: "password"},
			{Name: "confirm_password", Type: "password"},
			{Name: "name", Type: "text"},
		},
	}

	result := fa.Analyze(form, "https://example.com/signup")

	if result.FormType != FormTypeSignup {
		t.Errorf("FormType = %q, want %q", result.FormType, FormTypeSignup)
	}
	if !result.IsSignup {
		t.Error("IsSignup should be true")
	}
}

func TestFormAnalyzer_Analyze_SearchForm(t *testing.T) {
	fa := NewFormAnalyzer()

	form := FormInfo{
		Action: "/search",
		Method: "GET",
		Inputs: []InputInfo{
			{Name: "q", Type: "search"},
		},
	}

	result := fa.Analyze(form, "https://example.com")

	if result.FormType != FormTypeSearch {
		t.Errorf("FormType = %q, want %q", result.FormType, FormTypeSearch)
	}
	if !result.IsSearch {
		t.Error("IsSearch should be true")
	}
}

func TestFormAnalyzer_Analyze_ContactForm(t *testing.T) {
	fa := NewFormAnalyzer()

	form := FormInfo{
		Action: "/contact",
		Method: "POST",
		Inputs: []InputInfo{
			{Name: "email", Type: "email"},
			{Name: "message", Type: "textarea"},
		},
	}

	result := fa.Analyze(form, "https://example.com/contact")

	if result.FormType != FormTypeContact {
		t.Errorf("FormType = %q, want %q", result.FormType, FormTypeContact)
	}
	if !result.IsContact {
		t.Error("IsContact should be true")
	}
}

func TestFormAnalyzer_Analyze_PaymentForm(t *testing.T) {
	fa := NewFormAnalyzer()

	form := FormInfo{
		Action: "/checkout",
		Method: "POST",
		Inputs: []InputInfo{
			{Name: "card_number", Type: "text"},
			{Name: "expiry", Type: "text"},
			{Name: "cvv", Type: "text"},
			{Name: "billing_address", Type: "text"},
		},
	}

	result := fa.Analyze(form, "https://example.com/checkout")

	if result.FormType != FormTypePayment {
		t.Errorf("FormType = %q, want %q", result.FormType, FormTypePayment)
	}
	if !result.IsPayment {
		t.Error("IsPayment should be true")
	}
}

func TestFormAnalyzer_Analyze_UploadForm(t *testing.T) {
	fa := NewFormAnalyzer()

	form := FormInfo{
		Action:  "/upload",
		Method:  "POST",
		Enctype: "multipart/form-data",
		Inputs: []InputInfo{
			{Name: "file", Type: "file"},
			{Name: "description", Type: "text"},
		},
	}

	result := fa.Analyze(form, "https://example.com/upload")

	if result.FormType != FormTypeUpload {
		t.Errorf("FormType = %q, want %q", result.FormType, FormTypeUpload)
	}
	if !result.IsUpload {
		t.Error("IsUpload should be true")
	}
}

func TestFormAnalyzer_Analyze_CSRF(t *testing.T) {
	fa := NewFormAnalyzer()

	tests := []struct {
		name      string
		inputName string
		wantCSRF  bool
	}{
		{"csrf", "csrf", true},
		{"_csrf", "_csrf", true},
		{"csrftoken", "csrftoken", true},
		{"csrf_token", "csrf_token", true},
		{"csrfmiddlewaretoken", "csrfmiddlewaretoken", true},
		{"authenticity_token", "authenticity_token", true},
		{"_token", "_token", true},
		{"xsrf", "xsrf", true},
		{"antiforgery", "antiforgery", true},
		{"regular_field", "regular_field", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := FormInfo{
				Action: "/form",
				Method: "POST",
				Inputs: []InputInfo{
					{Name: tt.inputName, Type: "hidden", Value: "token"},
				},
			}

			result := fa.Analyze(form, "https://example.com")
			if result.HasCSRF != tt.wantCSRF {
				t.Errorf("HasCSRF = %v, want %v", result.HasCSRF, tt.wantCSRF)
			}
		})
	}
}

func TestFormAnalyzer_Analyze_Captcha(t *testing.T) {
	fa := NewFormAnalyzer()

	tests := []struct {
		name        string
		input       InputInfo
		wantCaptcha bool
	}{
		{"recaptcha class", InputInfo{Name: "response", Class: "g-recaptcha-response"}, true},
		{"hcaptcha", InputInfo{Name: "h-captcha-response"}, true},
		{"captcha id", InputInfo{Name: "answer", ID: "captcha-input"}, true},
		{"turnstile", InputInfo{Name: "cf-turnstile-response"}, true},
		{"no captcha", InputInfo{Name: "username"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := FormInfo{
				Action: "/form",
				Method: "POST",
				Inputs: []InputInfo{tt.input},
			}

			result := fa.Analyze(form, "https://example.com")
			if result.HasCaptcha != tt.wantCaptcha {
				t.Errorf("HasCaptcha = %v, want %v", result.HasCaptcha, tt.wantCaptcha)
			}
		})
	}
}

func TestFormAnalyzer_Analyze_Complexity(t *testing.T) {
	fa := NewFormAnalyzer()

	tests := []struct {
		name           string
		form           FormInfo
		minComplexity  int
	}{
		{
			name: "simple form",
			form: FormInfo{
				Action: "/search",
				Method: "GET",
				Inputs: []InputInfo{{Name: "q", Type: "text"}},
			},
			minComplexity: 1,
		},
		{
			name: "complex form",
			form: FormInfo{
				Action:  "/register",
				Method:  "POST",
				Enctype: "multipart/form-data",
				Inputs: []InputInfo{
					{Name: "username", Type: "text", Required: true, Pattern: "[a-z]+"},
					{Name: "email", Type: "email", Required: true},
					{Name: "password", Type: "password", Required: true},
					{Name: "avatar", Type: "file"},
					{Name: "csrf", Type: "hidden"},
				},
			},
			minComplexity: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fa.Analyze(tt.form, "https://example.com")
			if result.Complexity < tt.minComplexity {
				t.Errorf("Complexity = %d, want >= %d", result.Complexity, tt.minComplexity)
			}
		})
	}
}

func TestFormAnalyzer_GeneratePayload(t *testing.T) {
	fa := NewFormAnalyzer()

	form := FormInfo{
		Action: "/form",
		Method: "POST",
		Inputs: []InputInfo{
			{Name: "email", Type: "email"},
			{Name: "password", Type: "password"},
			{Name: "name", Type: "text"},
			{Name: "phone", Type: "tel"},
			{Name: "website", Type: "url"},
			{Name: "age", Type: "number"},
			{Name: "message", Type: "textarea"},
			{Name: "date", Type: "date"},
			{Name: "agree", Type: "checkbox", Value: "yes"},
			{Name: "csrf", Type: "hidden", Value: "token123"},
			{Name: "disabled_field", Type: "text", Disabled: true},
		},
	}

	payload := fa.GeneratePayload(form)

	if payload["email"] != "test@example.com" {
		t.Errorf("email = %q", payload["email"])
	}
	if payload["password"] != "TestPassword123!" {
		t.Errorf("password = %q", payload["password"])
	}
	if payload["csrf"] != "token123" {
		t.Errorf("csrf = %q", payload["csrf"])
	}
	if _, exists := payload["disabled_field"]; exists {
		t.Error("disabled field should not be in payload")
	}
}

// =============================================================================
// JSParser Tests
// =============================================================================

func TestNewJSParser(t *testing.T) {
	p := NewJSParser()
	if p == nil {
		t.Fatal("NewJSParser() returned nil")
	}
}

func TestJSParser_Parse_URLs(t *testing.T) {
	p := NewJSParser()

	js := `
		const apiURL = "https://api.example.com/v1/users";
		const endpoint = "/api/products";
		const versioned = "/v2/orders";
		const other = "not a url";
	`

	result := p.Parse(js)

	if len(result.URLs) < 3 {
		t.Errorf("len(URLs) = %d, want >= 3", len(result.URLs))
	}
}

func TestJSParser_Parse_APIEndpoints_Fetch(t *testing.T) {
	p := NewJSParser()

	js := `
		fetch('/api/users');
		fetch('/api/posts', { method: 'POST', body: JSON.stringify(data) });
	`

	result := p.Parse(js)

	if len(result.APIEndpoints) < 2 {
		t.Errorf("len(APIEndpoints) = %d, want >= 2", len(result.APIEndpoints))
	}

	methods := make(map[string]bool)
	for _, ep := range result.APIEndpoints {
		methods[ep.Method] = true
	}

	if !methods["GET"] {
		t.Error("expected GET endpoint")
	}
	if !methods["POST"] {
		t.Error("expected POST endpoint")
	}
}

func TestJSParser_Parse_APIEndpoints_Axios(t *testing.T) {
	p := NewJSParser()

	js := `
		axios.get('/api/users');
		axios.post('/api/users', userData);
		axios.put('/api/users/1', userData);
		axios.delete('/api/users/1');
		axios.patch('/api/users/1', changes);
	`

	result := p.Parse(js)

	if len(result.APIEndpoints) != 5 {
		t.Errorf("len(APIEndpoints) = %d, want 5", len(result.APIEndpoints))
	}

	methods := make(map[string]bool)
	for _, ep := range result.APIEndpoints {
		methods[ep.Method] = true
	}

	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		if !methods[method] {
			t.Errorf("expected %s endpoint", method)
		}
	}
}

func TestJSParser_Parse_APIEndpoints_jQuery(t *testing.T) {
	p := NewJSParser()

	js := `
		$.get('/api/data');
		$.post('/api/submit', formData);
		$.ajax({ url: '/api/custom', method: 'PUT' });
	`

	result := p.Parse(js)

	if len(result.APIEndpoints) < 3 {
		t.Errorf("len(APIEndpoints) = %d, want >= 3", len(result.APIEndpoints))
	}
}

func TestJSParser_Parse_WebSockets(t *testing.T) {
	p := NewJSParser()

	js := `
		const ws = new WebSocket('wss://example.com/ws');
		const wsUrl = "ws://localhost:8080/socket";
	`

	result := p.Parse(js)

	if len(result.WebSockets) != 2 {
		t.Errorf("len(WebSockets) = %d, want 2", len(result.WebSockets))
	}
}

func TestJSParser_Parse_Secrets(t *testing.T) {
	p := NewJSParser()

	js := `
		const config = {
			api_key: "sk_live_1234567890abcdef",
			secret_key: "secret_1234567890abcdef",
			password: "mysecretpassword",
		};
		const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U";
	`

	result := p.Parse(js)

	if len(result.Secrets) < 3 {
		t.Errorf("len(Secrets) = %d, want >= 3", len(result.Secrets))
	}
}

func TestJSParser_Parse_Secrets_FalsePositives(t *testing.T) {
	p := NewJSParser()

	js := `
		const config = {
			api_key: "your_api_key_here",
			secret: "example_secret",
			token: "<placeholder>",
			key: "test_key",
		};
	`

	result := p.Parse(js)

	// Should filter out placeholders
	if len(result.Secrets) > 0 {
		t.Errorf("len(Secrets) = %d, expected 0 (false positives)", len(result.Secrets))
	}
}

func TestJSParser_Parse_Routes(t *testing.T) {
	p := NewJSParser()

	js := `
		<Route path="/users" component={Users} />
		<Route path="/users/:id" component={UserDetail} />

		const routes = [
			{ path: '/home', component: Home },
			{ path: '/about', component: About },
		];

		app.route('/api/v1/users');
	`

	result := p.Parse(js)

	if len(result.Routes) < 5 {
		t.Errorf("len(Routes) = %d, want >= 5", len(result.Routes))
	}
}

func TestExtractURLParams(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		minWant int
	}{
		{
			name:    "path params colon",
			url:     "/users/:id/posts/:postId",
			minWant: 2,
		},
		{
			name:    "path params braces",
			url:     "/users/{userId}/orders/{orderId}",
			minWant: 2,
		},
		{
			name:    "template literal",
			url:     "/users/${userId}",
			minWant: 1, // at least userId from ${} pattern
		},
		{
			name:    "query params",
			url:     "/search?q=test&page=1&limit=10",
			minWant: 3,
		},
		{
			name:    "no params",
			url:     "/api/users",
			minWant: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := extractURLParams(tt.url)
			if len(params) < tt.minWant {
				t.Errorf("len(params) = %d, want >= %d", len(params), tt.minWant)
			}
		})
	}
}

// =============================================================================
// APIParser Tests
// =============================================================================

func TestNewAPIParser(t *testing.T) {
	p := NewAPIParser()
	if p == nil {
		t.Fatal("NewAPIParser() returned nil")
	}
}

func TestAPIParser_ParseFromResponse_JSON(t *testing.T) {
	p := NewAPIParser()

	body := []byte(`{
		"users": [
			{"id": 1, "profile_url": "/api/users/1"}
		],
		"_links": {
			"self": {"href": "/api/users"},
			"next": {"href": "/api/users?page=2", "method": "GET"}
		}
	}`)

	info := p.ParseFromResponse("https://api.example.com/v1/users", "application/json", body)

	if info.Type != APITypeREST {
		t.Errorf("Type = %q, want %q", info.Type, APITypeREST)
	}
	if info.Version != "1" {
		t.Errorf("Version = %q, want 1", info.Version)
	}
	if len(info.Endpoints) == 0 {
		t.Error("expected endpoints from HATEOAS links")
	}
}

func TestAPIParser_ParseFromResponse_XML(t *testing.T) {
	p := NewAPIParser()

	body := []byte(`<soap:Envelope></soap:Envelope>`)

	info := p.ParseFromResponse("https://api.example.com/soap", "application/xml", body)

	if info.Type != APITypeSOAP {
		t.Errorf("Type = %q, want %q", info.Type, APITypeSOAP)
	}
}

func TestAPIParser_detectVersion(t *testing.T) {
	p := NewAPIParser()

	tests := []struct {
		url     string
		version string
	}{
		{"https://api.example.com/v1/users", "1"},
		{"https://api.example.com/v2.1/users", "2.1"},
		{"https://api.example.com/api/v3/users", "3"},
		{"https://api.example.com/users?version=4", "4"},
		{"https://api.example.com/users", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			version := p.detectVersion(tt.url)
			if version != tt.version {
				t.Errorf("detectVersion(%q) = %q, want %q", tt.url, version, tt.version)
			}
		})
	}
}

func TestAPIParser_extractBaseURL(t *testing.T) {
	p := NewAPIParser()

	tests := []struct {
		url    string
		base   string
	}{
		{"https://api.example.com/api/users/1", "https://api.example.com/api"},
		{"https://api.example.com/v1/users", "https://api.example.com/v1"},
		{"https://api.example.com/rest/users", "https://api.example.com/rest"},
		{"https://api.example.com/users", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			base := p.extractBaseURL(tt.url)
			if base != tt.base {
				t.Errorf("extractBaseURL(%q) = %q, want %q", tt.url, base, tt.base)
			}
		})
	}
}

func TestAPIParser_ParseOpenAPISpec(t *testing.T) {
	p := NewAPIParser()

	spec := []byte(`{
		"openapi": "3.0.0",
		"servers": [{"url": "https://api.example.com/v1"}],
		"paths": {
			"/users": {
				"get": {
					"parameters": [
						{"name": "page", "in": "query"},
						{"name": "limit", "in": "query"}
					]
				},
				"post": {}
			},
			"/users/{id}": {
				"get": {
					"parameters": [
						{"name": "id", "in": "path"}
					]
				},
				"put": {},
				"delete": {}
			}
		}
	}`)

	info, err := p.ParseOpenAPISpec(spec)
	if err != nil {
		t.Fatalf("ParseOpenAPISpec() error = %v", err)
	}

	if info.Type != APITypeREST {
		t.Errorf("Type = %q, want %q", info.Type, APITypeREST)
	}
	if info.Version != "3.0.0" {
		t.Errorf("Version = %q, want 3.0.0", info.Version)
	}
	if info.BaseURL != "https://api.example.com/v1" {
		t.Errorf("BaseURL = %q", info.BaseURL)
	}
	if len(info.Endpoints) != 5 {
		t.Errorf("len(Endpoints) = %d, want 5", len(info.Endpoints))
	}
}

func TestAPIParser_ParseOpenAPISpec_Swagger2(t *testing.T) {
	p := NewAPIParser()

	spec := []byte(`{
		"swagger": "2.0",
		"host": "api.example.com",
		"basePath": "/v2",
		"schemes": ["https"],
		"paths": {
			"/users": {
				"get": {}
			}
		}
	}`)

	info, err := p.ParseOpenAPISpec(spec)
	if err != nil {
		t.Fatalf("ParseOpenAPISpec() error = %v", err)
	}

	if info.Version != "2.0" {
		t.Errorf("Version = %q, want 2.0", info.Version)
	}
	if info.BaseURL != "https://api.example.com/v2" {
		t.Errorf("BaseURL = %q", info.BaseURL)
	}
}

func TestAPIParser_ParseGraphQLSchema(t *testing.T) {
	p := NewAPIParser()

	schema := `
		type Query {
			users: [User]
			user(id: ID!): User
			posts: [Post]
		}

		type Mutation {
			createUser(input: CreateUserInput!): User
			deleteUser(id: ID!): Boolean
		}
	`

	info := p.ParseGraphQLSchema(schema)

	if info.Type != APITypeGraphQL {
		t.Errorf("Type = %q, want %q", info.Type, APITypeGraphQL)
	}

	// Should have endpoints for queries and mutations
	if len(info.Endpoints) < 5 {
		t.Errorf("len(Endpoints) = %d, want >= 5", len(info.Endpoints))
	}

	// All should point to /graphql with POST method
	for _, ep := range info.Endpoints {
		if ep.URL != "/graphql" {
			t.Errorf("endpoint URL = %q, want /graphql", ep.URL)
		}
		if ep.Method != "POST" {
			t.Errorf("endpoint Method = %q, want POST", ep.Method)
		}
	}
}

func TestIsURLField(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"url", true},
		{"href", true},
		{"link", true},
		{"uri", true},
		{"endpoint", true},
		{"path", true},
		{"resource", true},
		{"location", true},
		{"redirect_url", true},
		{"profile_url", true},
		{"name", false},
		{"id", false},
		{"email", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := isURLField(tt.key); got != tt.want {
				t.Errorf("isURLField(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestIsValidAPIURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://api.example.com", true},
		{"http://localhost:8080", true},
		{"/api/users", true},
		{"/v1/orders", true},
		{"", false},
		{"not a url", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := isValidAPIURL(tt.url); got != tt.want {
				t.Errorf("isValidAPIURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestIsHTTPMethod(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"POST", true},
		{"PUT", true},
		{"DELETE", true},
		{"PATCH", true},
		{"HEAD", true},
		{"OPTIONS", true},
		{"CONNECT", false},
		{"TRACE", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := isHTTPMethod(tt.method); got != tt.want {
				t.Errorf("isHTTPMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"short", "*****"}, // len <= 8, all masked
		{"12345678", "********"}, // len == 8, all masked
		{"1234567890", "1234**7890"}, // len > 8, partial mask
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := maskSecret(tt.value)
			if got != tt.want {
				t.Errorf("maskSecret(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestTruncateContext(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ..."},
		{"  padded  ", 20, "padded"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateContext(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateContext(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestIsLikelyFalsePositive(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"your_api_key_here", true},
		{"example_secret", true},
		{"xxxxx", true},
		{"placeholder_value", true},
		{"dummy_token", true},
		{"test_key", true},
		{"changeme", true},
		{"<api_key>", true},
		{"{{token}}", true},
		{"${SECRET}", true},
		{"sk_live_1234567890", false},
		{"actual_secret_value_here", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := isLikelyFalsePositive(tt.value)
			if got != tt.want {
				t.Errorf("isLikelyFalsePositive(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
