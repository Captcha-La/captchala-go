// Package captchala provides a server-side SDK for validating Captcha tokens.
//
// Usage:
//
//	import captchala "github.com/Captcha-La/captchala-go"
//
//	client := captchala.NewClient("your_app_key", "your_app_secret")
//	result, err := client.Validate(token)
//	if err != nil {
//	    // handle error
//	}
//	if result.Valid {
//	    // verification passed
//	}
package captchala

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// MainAPIURL is the primary API endpoint for pt_ tokens
	MainAPIURL = "https://apiv1.captcha.la/v1/validate"
	// BackupAPIURL is the backup API endpoint for offline_ tokens
	BackupAPIURL = "https://fallbackapiv1.captchala.com/api/validate"
	// IssueAPIURL mints a one-time server_token (sct_) — POST /v1/server/challenge/issue
	IssueAPIURL = "https://apiv1.captcha.la/v1/server/challenge/issue"
	// ModerationCheckURL multi-modal content moderation — POST /v1/moderation/check
	ModerationCheckURL = "https://apiv1.captcha.la/v1/moderation/check"
	// ModerationTextURL plain-text moderation — POST /v1/moderation/text
	ModerationTextURL = "https://apiv1.captcha.la/v1/moderation/text"

	// Token prefixes
	PrefixMain    = "pt_"      // Main API token prefix
	PrefixOffline = "offline_" // Backup/offline token prefix
	PrefixClient  = "client_"  // Client-only token prefix
)

// Client is the Captchala SDK client
type Client struct {
	AppKey    string
	AppSecret string
	Timeout   time.Duration
	// BaseURL overrides the main API URL. Mainly used for tests.
	// If empty, MainAPIURL is used.
	BaseURL string
	// BackupURL overrides the backup API URL. Mainly used for tests.
	// If empty, BackupAPIURL is used.
	BackupURL string
	// IssueURL / ModerationCheckURL / ModerationTextURL override the
	// corresponding endpoint URLs. Mainly used for tests; leave empty
	// in production.
	IssueURL              string
	ModerationCheckURLStr string
	ModerationTextURLStr  string
	client                *http.Client
}

// ValidateResult contains the validation result
type ValidateResult struct {
	// Valid indicates if the token is valid
	Valid bool `json:"valid"`
	// Offline indicates if this was an offline verification
	Offline bool `json:"offline"`
	// ClientOnly indicates if this is a client-only token (cannot be verified server-side)
	ClientOnly bool `json:"client_only"`
	// ChallengeID is the challenge identifier
	ChallengeID string `json:"challenge_id,omitempty"`
	// Action is the business action associated with the token
	Action string `json:"action,omitempty"`
	// UID is the user ID bound via bind_uid at server_token issuance time.
	// Use this to verify the pass_token was issued for the expected user.
	UID string `json:"uid,omitempty"`
	// Error contains the error message if validation failed
	Error string `json:"error,omitempty"`
	// Warning contains warning message (e.g., for client-only tokens)
	Warning string `json:"warning,omitempty"`
	// Degraded indicates a dg_ token issued under service degradation
	// (e.g. the app's quota is exhausted). Valid is ALWAYS false for degraded
	// tokens (secure default); whether to accept the request is YOUR decision:
	//
	//	if result.Valid || result.Degraded { /* allow */ }
	Degraded bool `json:"degraded,omitempty"`
	// DegradedReason is the degradation cause, e.g. "quota_exhausted"
	DegradedReason string `json:"degraded_reason,omitempty"`
	// CaptchaArgs is the solve-context echo (Geetest-style). All fields are
	// informational and MUST NOT be used for the pass/fail decision.
	CaptchaArgs CaptchaArgs `json:"captcha_args"`
}

// CaptchaArgs is the solve-time context the platform recorded, echoed back on
// validate (comparable to Geetest captcha_args). Informational only — never
// gate pass/fail on these. Fixed shape: every field is always present in the
// response, empty/zero when unknown.
type CaptchaArgs struct {
	// Platform: web / android / ios / flutter / windows / ...
	Platform string `json:"platform"`
	// UserIP recorded at solve time (informational).
	UserIP string `json:"user_ip"`
	// Referer is the web page URL the challenge was solved on (web only).
	Referer string `json:"referer"`
	// Pkg is the native app package / bundle id (native flows only).
	Pkg string `json:"pkg"`
	// SolvedAt is the solve completion time (unix seconds), 0 when unknown.
	SolvedAt int64 `json:"solved_at"`
	// RiskScore is the solve-time risk score (0-100, higher = riskier;
	// reCAPTCHA v3 score style). Informational — for your own secondary risk
	// decisions, not a pass/fail gate.
	RiskScore int64 `json:"risk_score"`
}

type apiResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"msg"`
	Data    map[string]interface{} `json:"data"`
}

type validateRequest struct {
	PassToken string `json:"pass_token"`
	KeepToken bool   `json:"keep_token"`
	// ClientIP is optional but recommended (omitempty): the end-user's IP from
	// your inbound request, used for additional risk checks. Safe to omit.
	ClientIP string `json:"client_ip,omitempty"`
}

// NewClient creates a new Captchala client with default timeout (5 seconds)
func NewClient(appKey, appSecret string) *Client {
	return NewClientWithTimeout(appKey, appSecret, 5*time.Second)
}

// NewClientWithTimeout creates a new Captchala client with custom timeout
func NewClientWithTimeout(appKey, appSecret string, timeout time.Duration) *Client {
	return &Client{
		AppKey:    appKey,
		AppSecret: appSecret,
		Timeout:   timeout,
		client:    &http.Client{Timeout: timeout},
	}
}

// Validate validates a captcha token and consumes it
func (c *Client) Validate(token string) (*ValidateResult, error) {
	return c.validateInternal(token, false, "")
}

// ValidateWithOptions validates a captcha token with options.
// If keepToken is true, the token will not be consumed and can be validated again.
func (c *Client) ValidateWithOptions(token string, keepToken bool) (*ValidateResult, error) {
	return c.validateInternal(token, keepToken, "")
}

// ValidateWithClientIP validates a token and forwards the end-user IP.
//
// The IP is optional but recommended — pass the end-user's IP from your
// inbound request; it is used for additional risk checks. Otherwise use
// Validate / ValidateWithOptions.
func (c *Client) ValidateWithClientIP(token string, keepToken bool, clientIP string) (*ValidateResult, error) {
	return c.validateInternal(token, keepToken, clientIP)
}

// validateInternal performs the actual token validation.
// clientIP is optional; when non-empty it is included in the request body
// so the backend can perform bind_ip verification.
func (c *Client) validateInternal(token string, keepToken bool, clientIP string) (*ValidateResult, error) {
	// Handle empty token
	if token == "" {
		return &ValidateResult{
			Valid: false,
			Error: "empty_token",
		}, errors.New("empty token")
	}

	// Handle client-only token (client_ prefix)
	// These tokens cannot be verified server-side
	if strings.HasPrefix(token, PrefixClient) {
		return &ValidateResult{
			Valid:      true,
			Offline:    true,
			ClientOnly: true,
			Warning:    "Client-only token cannot be verified server-side",
		}, nil
	}

	// Select API based on token prefix
	var apiURL string
	var isOffline bool

	if strings.HasPrefix(token, PrefixOffline) {
		// offline_ prefix -> use backup API
		apiURL = c.BackupURL
		if apiURL == "" {
			apiURL = BackupAPIURL
		}
		isOffline = true
	} else {
		// pt_ prefix or any other -> use main API
		apiURL = c.BaseURL
		if apiURL == "" {
			apiURL = MainAPIURL
		}
		isOffline = false
	}

	// Make request. clientIP is optional (omitempty): pass it if you want it
	// recorded, or leave it empty. Either way the dashboard does not gate
	// pass/fail on a caller IP (see validateRequest.ClientIP).
	reqBody := validateRequest{
		PassToken: token,
		KeepToken: keepToken,
		ClientIP:  clientIP,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Key", c.AppKey)
	req.Header.Set("X-App-Secret", c.AppSecret)

	resp, err := c.client.Do(req)
	if err != nil {
		return &ValidateResult{
			Valid:   false,
			Offline: isOffline,
			Error:   "request_failed",
		}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	// Parse response
	if apiResp.Code == 0 {
		valid := false
		if v, ok := apiResp.Data["valid"].(bool); ok {
			valid = v
		} else if v, ok := apiResp.Data["ok"].(bool); ok {
			valid = v
		}

		if valid {
			result := &ValidateResult{
				Valid:   true,
				Offline: isOffline,
			}
			if cid, ok := apiResp.Data["challenge_id"].(string); ok {
				result.ChallengeID = cid
			}
			if action, ok := apiResp.Data["action"].(string); ok {
				result.Action = action
			}
			if uid, ok := apiResp.Data["uid"].(string); ok {
				result.UID = uid
			}
			// captcha_args: solve-context echo (informational only).
			if ca, ok := apiResp.Data["captcha_args"].(map[string]interface{}); ok {
				if v, ok := ca["platform"].(string); ok {
					result.CaptchaArgs.Platform = v
				}
				if v, ok := ca["user_ip"].(string); ok {
					result.CaptchaArgs.UserIP = v
				}
				if v, ok := ca["referer"].(string); ok {
					result.CaptchaArgs.Referer = v
				}
				if v, ok := ca["pkg"].(string); ok {
					result.CaptchaArgs.Pkg = v
				}
				if v, ok := ca["solved_at"].(float64); ok {
					result.CaptchaArgs.SolvedAt = int64(v)
				}
				if v, ok := ca["risk_score"].(float64); ok {
					result.CaptchaArgs.RiskScore = int64(v)
				}
			}
			return result, nil
		}
	}

	// Degraded token (dg_ prefix): issued when the app's quota is exhausted so
	// end-user flows are not interrupted. Valid stays false; integrator decides.
	if degraded, ok := apiResp.Data["degraded"].(bool); ok && degraded {
		result := &ValidateResult{
			Valid:          false,
			Offline:        isOffline,
			Degraded:       true,
			DegradedReason: "quota_exhausted",
		}
		if reason, ok := apiResp.Data["reason"].(string); ok && reason != "" {
			result.DegradedReason = reason
		}
		return result, nil
	}

	// Validation failed
	errMsg := "unknown_error"
	if e, ok := apiResp.Data["error"].(string); ok {
		errMsg = e
	} else if apiResp.Message != "" {
		errMsg = apiResp.Message
	}

	return &ValidateResult{
		Valid:   false,
		Offline: isOffline,
		Error:   errMsg,
	}, errors.New(errMsg)
}

// IssueResult contains the server_token issuance result.
type IssueResult struct {
	OK        bool   `json:"ok"`
	Token     string `json:"token,omitempty"`      // sct_<hex>; pass to browser SDK as serverToken
	ExpiresIn int    `json:"expires_in,omitempty"` // seconds
	IssuedAt  int64  `json:"issued_at,omitempty"`  // unix seconds
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
}

// IssueOptions are optional knobs for IssueServerTokenWithOptions.
type IssueOptions struct {
	BindingIP string // End-user IP — backend rejects token if a different IP redeems it
	TTL       int    // Lifetime in seconds; server enforces a hard upper bound
	MaxUses   int    // SDK retry budget; verification is still single-pass
	BindUID   string // User ID to bind; pair with ValidateResult.UID on verify side
}

// IssueServerToken mints a one-time server_token for the given action.
// Hand the returned sct_ token to the browser SDK via the serverToken prop.
func (c *Client) IssueServerToken(action string) (*IssueResult, error) {
	return c.IssueServerTokenWithOptions(action, IssueOptions{})
}

// IssueServerTokenWithOptions is the same as IssueServerToken but lets you
// bind the token to an IP / UID and override TTL / max_uses.
func (c *Client) IssueServerTokenWithOptions(action string, opts IssueOptions) (*IssueResult, error) {
	if action == "" {
		return &IssueResult{OK: false, Error: "invalid_action", Message: "action is required"}, errors.New("invalid_action")
	}
	body := map[string]interface{}{"action": action}
	if opts.BindingIP != "" {
		body["binding_ip"] = opts.BindingIP
	}
	if opts.TTL > 0 {
		body["ttl"] = opts.TTL
	}
	if opts.MaxUses > 0 {
		body["max_uses"] = opts.MaxUses
	}
	if opts.BindUID != "" {
		body["bind_uid"] = opts.BindUID
	}

	url := c.IssueURL
	if url == "" {
		url = IssueAPIURL
	}
	resp, err := c.requestJSON(url, body)
	if err != nil {
		return &IssueResult{OK: false, Error: "request_failed", Message: err.Error()}, err
	}
	if resp.Code == 0 {
		if tok, ok := resp.Data["server_token"].(string); ok && tok != "" {
			r := &IssueResult{OK: true, Token: tok}
			if v, ok := resp.Data["expires_in"].(float64); ok {
				r.ExpiresIn = int(v)
			}
			if v, ok := resp.Data["issued_at"].(float64); ok {
				r.IssuedAt = int64(v)
			}
			return r, nil
		}
	}
	errCode := "unknown_error"
	if e, ok := resp.Data["error"].(string); ok {
		errCode = e
	} else if resp.Message != "" {
		errCode = resp.Message
	}
	return &IssueResult{OK: false, Error: errCode, Message: resp.Message}, errors.New(errCode)
}

// ModerationItem is one entry in a multi-modal moderation request.
// Use either Text (with Type="text") or ImageURL (with Type="image_url").
type ModerationItem struct {
	Type     string         `json:"type"` // "text" or "image_url"
	Text     string         `json:"text,omitempty"`
	ImageURL map[string]any `json:"image_url,omitempty"` // {"url": "https://..."}
}

// TextItem returns a {type:text, text} item ready for ModerationCheck input.
func TextItem(text string) ModerationItem {
	return ModerationItem{Type: "text", Text: text}
}

// ImageURLItem returns a {type:image_url, image_url:{url}} item.
func ImageURLItem(url string) ModerationItem {
	return ModerationItem{Type: "image_url", ImageURL: map[string]any{"url": url}}
}

// ModerationResult contains the moderation verdict.
type ModerationResult struct {
	OK          bool            `json:"ok"`
	Flagged     bool            `json:"flagged"`
	Categories  map[string]bool `json:"categories,omitempty"`   // category name → tripped
	ContentType string          `json:"content_type,omitempty"` // "text" / "image" / "mixed"
	Raw         map[string]any  `json:"raw,omitempty"`          // full upstream payload
	Error       string          `json:"error,omitempty"`
	Message     string          `json:"message,omitempty"`
}

// HasCategory reports true if any of the named categories tripped.
func (r *ModerationResult) HasCategory(names ...string) bool {
	for _, n := range names {
		if r.Categories[n] {
			return true
		}
	}
	return false
}

// ModerationCheck does multi-modal content moderation. Pass a slice of
// ModerationItem (text and/or image_url entries). userID is optional.
func (c *Client) ModerationCheck(input []ModerationItem, userID string) (*ModerationResult, error) {
	if len(input) == 0 {
		return &ModerationResult{OK: false, Error: "empty_input", Message: "input is required"}, errors.New("empty_input")
	}
	body := map[string]interface{}{
		"app_key":    c.AppKey,
		"app_secret": c.AppSecret,
		"input":      input,
	}
	if userID != "" {
		body["user_id"] = userID
	}
	url := c.ModerationCheckURLStr
	if url == "" {
		url = ModerationCheckURL
	}
	return c.parseModeration(c.requestJSON(url, body))
}

// ModerationText is a convenience wrapper for plain-text moderation.
func (c *Client) ModerationText(text, userID string) (*ModerationResult, error) {
	if text == "" {
		return &ModerationResult{OK: false, Error: "empty_text", Message: "text is required"}, errors.New("empty_text")
	}
	body := map[string]interface{}{
		"app_key":    c.AppKey,
		"app_secret": c.AppSecret,
		"text":       text,
	}
	if userID != "" {
		body["user_id"] = userID
	}
	url := c.ModerationTextURLStr
	if url == "" {
		url = ModerationTextURL
	}
	return c.parseModeration(c.requestJSON(url, body))
}

func (c *Client) parseModeration(resp *apiResponse, err error) (*ModerationResult, error) {
	if err != nil {
		return &ModerationResult{OK: false, Error: "request_failed", Message: err.Error()}, err
	}
	if resp.Code == 0 {
		r := &ModerationResult{OK: true, Raw: resp.Data}
		if v, ok := resp.Data["flagged"].(bool); ok {
			r.Flagged = v
		}
		if v, ok := resp.Data["content_type"].(string); ok {
			r.ContentType = v
		}
		if v, ok := resp.Data["categories"].(map[string]any); ok {
			r.Categories = make(map[string]bool, len(v))
			for k, val := range v {
				if b, ok := val.(bool); ok {
					r.Categories[k] = b
				}
			}
		}
		return r, nil
	}
	errCode := "unknown_error"
	if e, ok := resp.Data["error"].(string); ok {
		errCode = e
	} else if resp.Message != "" {
		errCode = resp.Message
	}
	return &ModerationResult{OK: false, Error: errCode, Message: resp.Message}, errors.New(errCode)
}

// requestJSON does the shared POST-JSON-with-headers dance and decodes
// the standard {code, msg, data} envelope.
func (c *Client) requestJSON(url string, body interface{}) (*apiResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Key", c.AppKey)
	req.Header.Set("X-App-Secret", c.AppSecret)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out apiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
