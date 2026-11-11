package captchala

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// closedURL is a localhost URL nothing listens on (connection refused).
const closedURL = "http://127.0.0.1:1"

// anyStatusServer answers every request with the given status and a
// non-JSON body: for reachability purposes any HTTP response counts.
func anyStatusServer(status int, hits *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			*hits++
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte("nope"))
	}))
}

func TestClientOnlyTokenAcceptedWhenBothAPIsUnreachable(t *testing.T) {
	for _, tok := range []string{"client_abc123xyz", "co_abc123xyz"} {
		client := NewClient("test_key", "test_secret")
		client.BaseURL = closedURL
		client.BackupURL = "http://127.0.0.1:2"
		result, err := client.Validate(tok)

		if err != nil {
			t.Errorf("%s: expected no error, got %v", tok, err)
		}
		if !result.Valid {
			t.Errorf("%s: expected client-only token to be valid during total outage", tok)
		}
		if !result.Offline {
			t.Errorf("%s: expected client-only token to be marked as offline", tok)
		}
		if !result.ClientOnly {
			t.Errorf("%s: expected client-only token to be marked as client_only", tok)
		}
		if result.Warning == "" {
			t.Errorf("%s: expected warning for client-only token", tok)
		}
	}
}

func TestClientOnlyTokenRejectedWhenMainReachable(t *testing.T) {
	for _, status := range []int{200, 400, 503} {
		for _, tok := range []string{"client_x", "co_x"} {
			hits := 0
			srv := anyStatusServer(status, &hits)
			client := NewClient("k", "s")
			client.BaseURL = srv.URL
			client.BackupURL = closedURL
			result, err := client.Validate(tok)
			srv.Close()

			if err == nil {
				t.Errorf("%s/%d: expected error", tok, status)
			}
			if result.Valid {
				t.Errorf("%s/%d: client-only token must be rejected when main API is reachable", tok, status)
			}
			if !result.ClientOnly {
				t.Errorf("%s/%d: expected ClientOnly=true", tok, status)
			}
			if result.Error != ErrClientOnlyRejected {
				t.Errorf("%s/%d: expected error %q, got %q", tok, status, ErrClientOnlyRejected, result.Error)
			}
			if hits != 1 {
				t.Errorf("%s/%d: expected exactly 1 probe, got %d", tok, status, hits)
			}
		}
	}
}

func TestClientOnlyTokenRejectedWhenOnlyBackupReachable(t *testing.T) {
	hits := 0
	srv := anyStatusServer(503, &hits)
	defer srv.Close()
	client := NewClient("k", "s")
	client.BaseURL = closedURL
	client.BackupURL = srv.URL

	result, err := client.Validate("client_x")
	if err == nil || result.Valid {
		t.Fatalf("expected rejection when backup is reachable, got %+v err=%v", result, err)
	}
	if !result.ClientOnly || result.Error != ErrClientOnlyRejected {
		t.Errorf("unexpected result %+v", result)
	}
	if hits != 1 {
		t.Errorf("expected backup to be probed once, got %d", hits)
	}
}

func TestClientOnlyProbeTimeoutCountsAsUnreachable(t *testing.T) {
	old := probeTimeout
	probeTimeout = 100 * time.Millisecond
	defer func() { probeTimeout = old }()

	block := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer slow.Close()
	defer close(block)

	client := NewClient("k", "s")
	client.BaseURL = slow.URL
	client.BackupURL = closedURL
	result, err := client.Validate("client_x")
	if err != nil || !result.Valid || !result.ClientOnly {
		t.Fatalf("expected acceptance when probes time out, got %+v err=%v", result, err)
	}
}

func TestClientOnlyReachabilityIsCached(t *testing.T) {
	hits := 0
	srv := anyStatusServer(200, &hits)
	defer srv.Close()
	client := NewClient("k", "s")
	client.BaseURL = srv.URL
	for i := 0; i < 5; i++ {
		result, _ := client.Validate("client_x")
		if result.Valid {
			t.Fatal("expected rejection")
		}
	}
	if hits != 1 {
		t.Errorf("expected 1 probe thanks to the cache, got %d", hits)
	}
}

func TestIsClientOnlyToken(t *testing.T) {
	cases := map[string]bool{
		"client_a": true, "co_a": true, "pt_a": false, "offline_a": false, "dg_a": false, "cox": false,
	}
	for tok, want := range cases {
		if got := IsClientOnlyToken(tok); got != want {
			t.Errorf("IsClientOnlyToken(%q) = %v, want %v", tok, got, want)
		}
	}
}

func TestValidateNonJSON5xxReturnsResult(t *testing.T) {
	srv := anyStatusServer(502, nil)
	defer srv.Close()
	client := NewClient("k", "s")
	client.BaseURL = srv.URL

	result, err := client.Validate("pt_x")
	if err == nil {
		t.Fatal("expected error")
	}
	if result == nil {
		t.Fatal("expected non-nil result for non-JSON 5xx")
	}
	if result.Valid || result.Error != "http_502" {
		t.Errorf("unexpected result %+v", result)
	}
}

func TestValidateNon2xxNeverValid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"code":0,"data":{"valid":true}}`))
	}))
	defer srv.Close()
	client := NewClient("k", "s")
	client.BaseURL = srv.URL
	result, err := client.Validate("pt_x")
	if err == nil || result == nil || result.Valid {
		t.Fatalf("non-2xx must not validate, got %+v err=%v", result, err)
	}
}

func TestStructLiteralClientDoesNotPanic(t *testing.T) {
	srv := newMockServer(t, map[string]interface{}{"code": 0, "data": map[string]interface{}{"valid": true}}, nil, nil)
	defer srv.Close()
	client := &Client{AppKey: "k", AppSecret: "s", BaseURL: srv.URL}
	result, err := client.Validate("pt_x")
	if err != nil || !result.Valid {
		t.Fatalf("unexpected %+v err=%v", result, err)
	}
}

func TestIssueNonJSONResponse(t *testing.T) {
	srv := anyStatusServer(502, nil)
	defer srv.Close()
	client := NewClient("k", "s")
	client.IssueURL = srv.URL
	r, err := client.IssueServerToken("login")
	if err == nil || r == nil || r.OK {
		t.Fatalf("expected failure result, got %+v err=%v", r, err)
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

	if PrefixClientWeb != "co_" {
		t.Errorf("Expected PrefixClientWeb to be 'co_', got '%s'", PrefixClientWeb)
	}

	if PrefixDegraded != "dg_" {
		t.Errorf("Expected PrefixDegraded to be 'dg_', got '%s'", PrefixDegraded)
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
	client.BaseURL = closedURL
	client.BackupURL = closedURL

	for i := 0; i < b.N; i++ {
		client.Validate("client_abc123xyz")
	}
}

func resetProbeState() {
	reachabilityCache.Lock()
	reachabilityCache.m = map[string]probeEntry{}
	reachabilityCache.Unlock()
}

func TestClientOnlyUnreachableIsCachedBriefly(t *testing.T) {
	resetProbeState()
	hits := 0
	// a server that is closed after creation: probes fail, and we count dial attempts via a transport
	client := NewClient("k", "s")
	client.BaseURL = "http://127.0.0.1:3"
	client.BackupURL = "http://127.0.0.1:4"
	client.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hits++
		return nil, io.ErrUnexpectedEOF
	})}
	for i := 0; i < 5; i++ {
		result, err := client.Validate("client_x")
		if err != nil || !result.Valid {
			t.Fatalf("expected acceptance during outage, got %+v err=%v", result, err)
		}
	}
	if hits != 2 {
		t.Errorf("expected one probe per endpoint then cached unreachable, got %d", hits)
	}
}

func TestRejectClientOnlyRefusesWithoutProbing(t *testing.T) {
	resetProbeState()
	for _, tok := range []string{"client_x", "co_x"} {
		hits := 0
		srv := anyStatusServer(200, &hits)
		client := NewClient("k", "s")
		client.BaseURL = srv.URL
		client.BackupURL = srv.URL
		client.RejectClientOnly = true
		result, err := client.Validate(tok)
		srv.Close()
		if err == nil || result.Valid || !result.ClientOnly || result.Error != ErrClientOnlyPolicy {
			t.Fatalf("%s: expected policy rejection, got %+v err=%v", tok, result, err)
		}
		if hits != 0 {
			t.Errorf("%s: expected no probe, got %d", tok, hits)
		}
	}
}

func TestClientOnlyProbeIsSingleFlight(t *testing.T) {
	resetProbeState()
	var mu sync.Mutex
	hits := 0
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		<-release
		w.WriteHeader(200)
	}))
	defer srv.Close()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := NewClient("k", "s")
			client.BaseURL = srv.URL
			if r, _ := client.Validate("client_x"); r.Valid {
				t.Error("expected rejection while the main API is reachable")
			}
		}()
	}
	time.Sleep(200 * time.Millisecond) // let every goroutine reach the probe
	close(release)
	wg.Wait()
	if hits != 1 {
		t.Errorf("expected a single probe for concurrent validations, got %d", hits)
	}
}

func TestIssueSendsBindIP(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"code":0,"data":{"server_token":"st_x","expires_in":300}}`))
	}))
	defer srv.Close()
	client := NewClient("k", "s")
	client.IssueURL = srv.URL
	if _, err := client.IssueServerTokenWithOptions("login", IssueOptions{BindingIP: "1.2.3.4"}); err != nil {
		t.Fatal(err)
	}
	if body["bind_ip"] != "1.2.3.4" {
		t.Errorf("expected bind_ip, got %v", body)
	}
	if _, old := body["binding_ip"]; old {
		t.Errorf("binding_ip should no longer be sent: %v", body)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
