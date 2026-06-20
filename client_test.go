package captchala

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestClientOnlyToken(t *testing.T) {
	client := NewClient("test_key", "test_secret")
	result, err := client.Validate("client_abc123xyz")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !result.Valid {
		t.Error("Expected client-only token to be valid")
	}

	if !result.Offline {
		t.Error("Expected client-only token to be marked as offline")
	}

	if !result.ClientOnly {
		t.Error("Expected client-only token to be marked as client_only")
	}

	if result.Warning == "" {
		t.Error("Expected warning for client-only token")
	}
}

func TestEmptyToken(t *testing.T) {
	client := NewClient("test_key", "test_secret")
	result, err := client.Validate("")

	if err == nil {
		t.Error("Expected error for empty token")
	}

	if result.Valid {
		t.Error("Expected empty token to be invalid")
	}

	if result.Error != "empty_token" {
		t.Errorf("Expected error 'empty_token', got '%s'", result.Error)
	}
}

func TestMainTokenTypeDetection(t *testing.T) {
	client := NewClient("test_key", "test_secret")
	// pt_ is the main API token prefix
	result, _ := client.Validate("pt_invalid_token")

	if result.ClientOnly {
		t.Error("pt_ token should not be client_only")
	}

	if result.Offline {
		t.Error("pt_ token should not be offline")
	}
}

func TestOfflineTokenTypeDetection(t *testing.T) {
	client := NewClient("test_key", "test_secret")
	result, _ := client.Validate("offline_invalid_token")

	if result.ClientOnly {
		t.Error("offline_ token should not be client_only")
	}

	if !result.Offline {
		t.Error("offline_ token should be marked as offline")
	}
}

func TestTokenPrefixConstants(t *testing.T) {
	if PrefixMain != "pt_" {
		t.Errorf("Expected PrefixMain to be 'pt_', got '%s'", PrefixMain)
	}

	if PrefixOffline != "offline_" {
		t.Errorf("Expected PrefixOffline to be 'offline_', got '%s'", PrefixOffline)
	}

	if PrefixClient != "client_" {
		t.Errorf("Expected PrefixClient to be 'client_', got '%s'", PrefixClient)
	}
}

func TestNewClientWithTimeout(t *testing.T) {
	client := NewClientWithTimeout("key", "secret", 10*time.Second)

	if client.Timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", client.Timeout)
	}
}

func TestRealValidation(t *testing.T) {
	appKey := os.Getenv("CAPTCHALA_APP_KEY")
	appSecret := os.Getenv("CAPTCHALA_APP_SECRET")
	token := os.Getenv("CAPTCHALA_TEST_TOKEN")

	if appKey == "" || appSecret == "" || token == "" {
		t.Skip("Real credentials not provided")
	}

	client := NewClient(appKey, appSecret)
	result, err := client.Validate(token)

	if err != nil && !result.Valid {
		t.Logf("Validation result: valid=%v, error=%s", result.Valid, result.Error)
	}
}

// --- Mock server based tests ---------------------------------------------------

// newMockServer returns a test server that records received body + headers
// and responds with the provided response payload.
func newMockServer(t *testing.T, responsePayload map[string]interface{}, captured *validateRequest, capturedHeaders *http.Header) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if captured != nil {
			_ = json.Unmarshal(body, captured)
		}
		if capturedHeaders != nil {
			h := r.Header.Clone()
			*capturedHeaders = h
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responsePayload)
	}))
}

func TestValidateMainHappyPath(t *testing.T) {
	var captured validateRequest
	var capturedHeaders http.Header
	srv := newMockServer(t, map[string]interface{}{
		"code": 0,
		"msg":  "ok",
		"data": map[string]interface{}{
			"valid":        true,
			"challenge_id": "ch_abc",
			"action":       "login",
			"uid":          "user_42",
		},
	}, &captured, &capturedHeaders)
	defer srv.Close()

	client := NewClient("k", "s")
	client.BaseURL = srv.URL

	result, err := client.Validate("pt_some_real_token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid=true, got %+v", result)
	}
	if result.ChallengeID != "ch_abc" {
		t.Errorf("expected challenge_id 'ch_abc', got %q", result.ChallengeID)
	}
	if result.Action != "login" {
		t.Errorf("expected action 'login', got %q", result.Action)
	}
	if result.UID != "user_42" {
		t.Errorf("expected uid 'user_42', got %q", result.UID)
	}
	if result.Offline {
		t.Error("pt_ token should not be offline")
	}
	// request body should NOT contain client_ip when not set
	if captured.ClientIP != "" {
		t.Errorf("expected empty client_ip in body, got %q", captured.ClientIP)
	}
	if captured.PassToken != "pt_some_real_token" {
		t.Errorf("unexpected pass_token: %q", captured.PassToken)
	}
	if capturedHeaders.Get("X-App-Key") != "k" || capturedHeaders.Get("X-App-Secret") != "s" {
		t.Errorf("auth headers not forwarded: %v", capturedHeaders)
	}
}

func TestValidateOfflineUsesBackupURL(t *testing.T) {
	srv := newMockServer(t, map[string]interface{}{
		"code": 0,
		"msg":  "ok",
		"data": map[string]interface{}{"valid": true},
	}, nil, nil)
	defer srv.Close()

	client := NewClient("k", "s")
	client.BackupURL = srv.URL

	result, err := client.Validate("offline_abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatal("expected valid=true")
	}
	if !result.Offline {
		t.Error("offline_ token should be marked offline")
	}
}

func TestValidateSendsClientIPWhenProvided(t *testing.T) {
	// client_ip is optional: sent when provided (used for risk checks).
	var captured validateRequest
	srv := newMockServer(t, map[string]interface{}{
		"code": 0,
		"msg":  "ok",
		"data": map[string]interface{}{"valid": true},
	}, &captured, nil)
	defer srv.Close()

	client := NewClient("k", "s")
	client.BaseURL = srv.URL

	result, err := client.ValidateWithClientIP("pt_xyz", false, "203.0.113.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Error("expected valid=true")
	}
	if captured.ClientIP != "203.0.113.9" {
		t.Errorf("client_ip should be sent when provided, got %q", captured.ClientIP)
	}
}

func TestValidateParsesCaptchaArgs(t *testing.T) {
	srv := newMockServer(t, map[string]interface{}{
		"code": 0,
		"msg":  "ok",
		"data": map[string]interface{}{
			"valid": true,
			"captcha_args": map[string]interface{}{
				"platform":  "web",
				"user_ip":   "198.51.100.7",
				"referer":   "https://shop.example.com/checkout",
				"pkg":       nil,
				"solved_at": float64(1750000000),
			},
		},
	}, nil, nil)
	defer srv.Close()

	client := NewClient("k", "s")
	client.BaseURL = srv.URL

	result, err := client.Validate("pt_xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CaptchaArgs.Platform != "web" {
		t.Errorf("platform: got %q", result.CaptchaArgs.Platform)
	}
	if result.CaptchaArgs.UserIP != "198.51.100.7" {
		t.Errorf("user_ip: got %q", result.CaptchaArgs.UserIP)
	}
	if result.CaptchaArgs.Referer != "https://shop.example.com/checkout" {
		t.Errorf("referer: got %q", result.CaptchaArgs.Referer)
	}
	if result.CaptchaArgs.SolvedAt != 1750000000 {
		t.Errorf("solved_at: got %d", result.CaptchaArgs.SolvedAt)
	}
}

func TestValidateWithClientIPEmptyOmitsField(t *testing.T) {
	// When clientIP == "", the JSON body must NOT contain a "client_ip" key.
	var rawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok","data":{"valid":true}}`))
	}))
	defer srv.Close()

	client := NewClient("k", "s")
	client.BaseURL = srv.URL

	_, err := client.ValidateWithClientIP("pt_xyz", true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(rawBody), "client_ip") {
		t.Errorf("client_ip should be omitted when empty, got body %s", string(rawBody))
	}
}

func TestValidateNetworkErrorReturnsRequestFailed(t *testing.T) {
	client := NewClient("k", "s")
	// Point to a closed port on localhost → connection refused
	client.BaseURL = "http://127.0.0.1:1"

	result, err := client.Validate("pt_something")
	if err == nil {
		t.Fatal("expected network error")
	}
	if result == nil {
		t.Fatal("expected non-nil result on network error")
	}
	if result.Valid {
		t.Error("expected invalid on network error")
	}
	if result.Error != "request_failed" {
		t.Errorf("expected error 'request_failed', got %q", result.Error)
	}
}

func TestValidateBackendErrorPropagates(t *testing.T) {
	srv := newMockServer(t, map[string]interface{}{
		"code": 0,
		"msg":  "",
		"data": map[string]interface{}{
			"valid": false,
			"error": "token_expired",
		},
	}, nil, nil)
	defer srv.Close()

	client := NewClient("k", "s")
	client.BaseURL = srv.URL

	result, err := client.Validate("pt_bad")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if result.Valid {
		t.Error("expected valid=false")
	}
	if result.Error != "token_expired" {
		t.Errorf("expected error 'token_expired', got %q", result.Error)
	}
}

func TestValidateWithOptionsKeepToken(t *testing.T) {
	var captured validateRequest
	srv := newMockServer(t, map[string]interface{}{
		"code": 0,
		"msg":  "ok",
		"data": map[string]interface{}{"valid": true},
	}, &captured, nil)
	defer srv.Close()

	client := NewClient("k", "s")
	client.BaseURL = srv.URL

	_, err := client.ValidateWithOptions("pt_x", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !captured.KeepToken {
		t.Error("expected keep_token=true in body")
	}
}

// Benchmark tests
func BenchmarkClientOnlyToken(b *testing.B) {
	client := NewClient("test_key", "test_secret")

	for i := 0; i < b.N; i++ {
		client.Validate("client_abc123xyz")
	}
}
