package parser

import (
	"strings"
)

// FormAnalyzer provides comprehensive form analysis.
type FormAnalyzer struct{}

// NewFormAnalyzer creates a new form analyzer.
func NewFormAnalyzer() *FormAnalyzer {
	return &FormAnalyzer{}
}

// AnalyzeResult contains detailed form analysis results.
type AnalyzeResult struct {
	Form       Form
	FormType   FormType
	HasCSRF    bool
	CSRFField  string
	HasCaptcha bool
	IsLogin    bool
	IsSignup   bool
	IsSearch   bool
	IsContact  bool
	IsPayment  bool
	IsUpload   bool
	Complexity int // 1-10 complexity score
}

// FormType represents the type of form.
type FormType string

const (
	FormTypeLogin    FormType = "login"
	FormTypeSignup   FormType = "signup"
	FormTypeSearch   FormType = "search"
	FormTypeContact  FormType = "contact"
	FormTypePayment  FormType = "payment"
	FormTypeUpload   FormType = "upload"
	FormTypeComment  FormType = "comment"
	FormTypeSettings FormType = "settings"
	FormTypeGeneric  FormType = "generic"
)

// Analyze performs comprehensive analysis of a form.
func (a *FormAnalyzer) Analyze(form FormInfo, pageURL string) *AnalyzeResult {
	result := &AnalyzeResult{
		Form: Form{
			URL:     pageURL,
			Action:  form.Action,
			Method:  form.Method,
			Enctype: form.Enctype,
			Inputs:  make([]FormInput, 0),
		},
	}

	// Convert inputs
	for _, input := range form.Inputs {
		formInput := FormInput{
			Name:        input.Name,
			Type:        input.Type,
			Value:       input.Value,
			Required:    input.Required,
			Placeholder: input.Placeholder,
			Pattern:     input.Pattern,
			MaxLength:   input.MaxLength,
			MinLength:   input.MinLength,
		}
		result.Form.Inputs = append(result.Form.Inputs, formInput)
	}

	// Detect CSRF token
	result.HasCSRF, result.CSRFField = a.detectCSRF(form.Inputs)
	result.Form.HasCSRF = result.HasCSRF

	// Detect form type
	result.FormType = a.detectFormType(form)

	// Set type flags
	result.IsLogin = result.FormType == FormTypeLogin
	result.IsSignup = result.FormType == FormTypeSignup
	result.IsSearch = result.FormType == FormTypeSearch
	result.IsContact = result.FormType == FormTypeContact
	result.IsPayment = result.FormType == FormTypePayment
	result.IsUpload = result.FormType == FormTypeUpload

	// Detect captcha
	result.HasCaptcha = a.detectCaptcha(form)

	// Calculate complexity
	result.Complexity = a.calculateComplexity(form)

	return result
}

// detectCSRF detects CSRF protection tokens.
func (a *FormAnalyzer) detectCSRF(inputs []InputInfo) (bool, string) {
	csrfPatterns := []string{
		"csrf",
		"_csrf",
		"csrftoken",
		"csrf_token",
		"csrfmiddlewaretoken",
		"__requestverificationtoken",
		"authenticity_token",
		"_token",
		"xsrf",
		"_xsrf",
		"antiforgery",
	}

	for _, input := range inputs {
		if input.Type != "hidden" {
			continue
		}

		nameLower := strings.ToLower(input.Name)
		for _, pattern := range csrfPatterns {
			if strings.Contains(nameLower, pattern) {
				return true, input.Name
			}
		}
	}

	return false, ""
}

// detectFormType determines the type of form.
func (a *FormAnalyzer) detectFormType(form FormInfo) FormType {
	inputNames := make([]string, 0)
	inputTypes := make(map[string]int)

	for _, input := range form.Inputs {
		inputNames = append(inputNames, strings.ToLower(input.Name))
		inputTypes[input.Type]++
	}

	allNames := strings.Join(inputNames, " ")
	actionLower := strings.ToLower(form.Action)

	// Login form
	loginIndicators := []string{"login", "signin", "sign-in", "log-in", "auth"}
	if hasPassword(inputTypes) && countInputs(inputTypes) <= 4 {
		for _, ind := range loginIndicators {
			if strings.Contains(allNames, ind) || strings.Contains(actionLower, ind) {
				return FormTypeLogin
			}
		}
		// Also check for username/password combo
		if strings.Contains(allNames, "password") &&
			(strings.Contains(allNames, "username") || strings.Contains(allNames, "email")) {
			return FormTypeLogin
		}
	}

	// Signup form
	signupIndicators := []string{"signup", "register", "sign-up", "create", "join"}
	if hasPassword(inputTypes) && countInputs(inputTypes) > 3 {
		for _, ind := range signupIndicators {
			if strings.Contains(allNames, ind) || strings.Contains(actionLower, ind) {
				return FormTypeSignup
			}
		}
		// Multiple fields with password and email/confirm
		if strings.Contains(allNames, "confirm") || strings.Contains(allNames, "password2") {
			return FormTypeSignup
		}
	}

	// Search form
	if inputTypes["search"] > 0 || strings.Contains(allNames, "search") || strings.Contains(allNames, "query") || strings.Contains(allNames, "q ") {
		return FormTypeSearch
	}

	// Contact form
	contactIndicators := []string{"contact", "message", "inquiry", "feedback"}
	if inputTypes["textarea"] > 0 {
		for _, ind := range contactIndicators {
			if strings.Contains(allNames, ind) || strings.Contains(actionLower, ind) {
				return FormTypeContact
			}
		}
		if strings.Contains(allNames, "email") && strings.Contains(allNames, "message") {
			return FormTypeContact
		}
	}

	// Payment form
	paymentIndicators := []string{"payment", "checkout", "card", "credit", "billing"}
	for _, ind := range paymentIndicators {
		if strings.Contains(allNames, ind) || strings.Contains(actionLower, ind) {
			return FormTypePayment
		}
	}

	// Upload form
	if inputTypes["file"] > 0 || form.Enctype == "multipart/form-data" {
		return FormTypeUpload
	}

	// Comment form
	if inputTypes["textarea"] > 0 && countInputs(inputTypes) <= 3 {
		if strings.Contains(allNames, "comment") || strings.Contains(actionLower, "comment") {
			return FormTypeComment
		}
	}

	// Settings form
	if strings.Contains(actionLower, "settings") || strings.Contains(actionLower, "profile") ||
		strings.Contains(actionLower, "preferences") {
		return FormTypeSettings
	}

	return FormTypeGeneric
}

// detectCaptcha checks for CAPTCHA elements.
func (a *FormAnalyzer) detectCaptcha(form FormInfo) bool {
	captchaPatterns := []string{
		"captcha",
		"recaptcha",
		"hcaptcha",
		"g-recaptcha",
		"h-captcha",
		"turnstile",
	}

	for _, input := range form.Inputs {
		nameLower := strings.ToLower(input.Name)
		idLower := strings.ToLower(input.ID)
		classLower := strings.ToLower(input.Class)

		for _, pattern := range captchaPatterns {
			if strings.Contains(nameLower, pattern) ||
				strings.Contains(idLower, pattern) ||
				strings.Contains(classLower, pattern) {
				return true
			}
		}
	}

	return false
}

// calculateComplexity calculates form complexity score (1-10).
func (a *FormAnalyzer) calculateComplexity(form FormInfo) int {
	score := 1

	// Number of inputs
	inputCount := len(form.Inputs)
	if inputCount > 2 {
		score++
	}
	if inputCount > 5 {
		score++
	}
	if inputCount > 10 {
		score++
	}

	// Input types
	hasHidden := false
	for _, input := range form.Inputs {
		switch input.Type {
		case "file":
			score++
		case "password":
			score++
		case "hidden":
			hasHidden = true
		}

		// Validation complexity
		if input.Required {
			score++
		}
		if input.Pattern != "" {
			score++
		}
	}

	if hasHidden {
		score++
	}

	// Method complexity
	if form.Method == "POST" {
		score++
	}

	// Enctype complexity
	if form.Enctype == "multipart/form-data" {
		score++
	}

	// Cap at 10
	if score > 10 {
		score = 10
	}

	// Ensure minimum of 1
	if score < 1 {
		score = 1
	}

	return score
}

func hasPassword(types map[string]int) bool {
	return types["password"] > 0
}

func countInputs(types map[string]int) int {
	total := 0
	for t, count := range types {
		if t != "hidden" && t != "submit" && t != "button" {
			total += count
		}
	}
	return total
}

// GeneratePayload generates test payloads for a form.
func (a *FormAnalyzer) GeneratePayload(form FormInfo) map[string]string {
	payload := make(map[string]string)

	for _, input := range form.Inputs {
		if input.Disabled {
			continue
		}

		name := input.Name
		if name == "" {
			continue
		}

		// Use existing value for hidden fields
		if input.Type == "hidden" {
			payload[name] = input.Value
			continue
		}

		// Generate appropriate values based on type
		switch input.Type {
		case "text", "search":
			payload[name] = generateTextValue(input)
		case "email":
			payload[name] = "test@example.com"
		case "password":
			payload[name] = "TestPassword123!"
		case "number":
			payload[name] = "42"
		case "tel":
			payload[name] = "+1234567890"
		case "url":
			payload[name] = "https://example.com"
		case "date":
			payload[name] = "2024-01-15"
		case "time":
			payload[name] = "12:00"
		case "datetime-local":
			payload[name] = "2024-01-15T12:00"
		case "checkbox", "radio":
			if input.Value != "" {
				payload[name] = input.Value
			} else {
				payload[name] = "on"
			}
		case "textarea":
			payload[name] = "This is a test message for form analysis."
		default:
			if input.Value != "" {
				payload[name] = input.Value
			} else {
				payload[name] = "test"
			}
		}
	}

	return payload
}

func generateTextValue(input InputInfo) string {
	nameLower := strings.ToLower(input.Name)

	if strings.Contains(nameLower, "name") {
		return "John Doe"
	}
	if strings.Contains(nameLower, "user") {
		return "testuser"
	}
	if strings.Contains(nameLower, "phone") {
		return "+1234567890"
	}
	if strings.Contains(nameLower, "address") {
		return "123 Test Street"
	}
	if strings.Contains(nameLower, "city") {
		return "Test City"
	}
	if strings.Contains(nameLower, "zip") || strings.Contains(nameLower, "postal") {
		return "12345"
	}
	if strings.Contains(nameLower, "country") {
		return "US"
	}

	if input.Placeholder != "" {
		return "test"
	}

	return "test"
}
