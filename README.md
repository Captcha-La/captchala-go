# Captchala Go SDK

Server-side SDK for validating Captcha tokens.

**English** | [简体中文](README_zh.md)

- [Website](https://captcha.la)
- [Documentation](https://docs.captcha.la/sdk/server-go)
- [Dashboard](https://dash.captcha.la)

## Installation

```bash
go get github.com/Captcha-La/captchala-go
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    captchala "github.com/Captcha-La/captchala-go"
)

func main() {
    // Create client
    client := captchala.NewClient("your_app_key", "your_app_secret")

    // Validate token
    result, err := client.Validate(token)
    if err != nil {
        log.Printf("Validation error: %v", err)
    }

    if result.Valid {
        fmt.Println("Verification passed")
        if result.Offline {
            fmt.Println("This was an offline verification")
        }
    } else {
        fmt.Printf("Verification failed: %s\n", result.Error)
    }
}
```

## API Reference

### `NewClient(appKey, appSecret string) *Client`

Create a client with default 5-second timeout.

### `NewClientWithTimeout(appKey, appSecret string, timeout time.Duration) *Client`

Create a client with custom timeout.

### `Client.Validate(token string) (*ValidateResult, error)`

Validate and consume a token.

### `Client.ValidateWithOptions(token string, keepToken bool) (*ValidateResult, error)`

Validate a token. If `keepToken` is `true`, the token won't be consumed and can be validated again.

### `Client.ValidateWithClientIP(token string, keepToken bool, clientIP string) (*ValidateResult, error)`

Validate a token and forward the end-user IP for `bind_ip` verification. If the
`pass_token` was issued with `bind_ip`, the backend compares this value against
the bound IP and rejects mismatches. Pass the real user IP extracted from YOUR
inbound request (e.g., the `X-Forwarded-For` head), not a proxy IP. When
`clientIP` is empty the field is omitted from the request body — behaving
identically to `ValidateWithOptions`.

```go
result, err := client.ValidateWithClientIP(token, false, userIP)
```

### `ValidateResult`

```go
type ValidateResult struct {
    Valid       bool   // Whether validation passed
    Offline     bool   // Whether this was offline verification
    ClientOnly  bool   // Whether this is a client-only token
    ChallengeID string // Challenge ID
    Action      string // Business action
    UID         string // User ID bound via bind_uid (for server-side identity check)
    Error       string // Error message
    Warning     string // Warning message
}
```

### Verifying `bind_uid`

If you issued the `server_token` with `bind_uid = "user_42"`, check the result:

```go
result, err := client.Validate(token)
if err == nil && result.Valid && result.UID != expectedUserID {
    // pass_token was issued for a different user — reject
}
```

### `Client.IssueServerToken(action string) (*IssueResult, error)`

Mint a one-time `sct_` server token with default options (no IP/UID binding,
default TTL/maxUses).

### `Client.IssueServerTokenWithOptions(action string, opts IssueOptions) (*IssueResult, error)`

Mint a server token with full control. Hand the returned `Token` to the
browser SDK as the `serverToken` prop — single-use, action-scoped,
optionally IP/UID-bound.

```go
issue, err := client.IssueServerTokenWithOptions("login", captchala.IssueOptions{
    BindingIP: userIP,    // backend rejects if a different IP redeems
    TTL:       300,       // seconds
    MaxUses:   5,         // SDK retry budget
    BindUID:   userID,    // pair with ValidateResult.UID on verify
})
if err != nil || !issue.OK {
    http.Error(w, issue.Error, http.StatusBadRequest)
    return
}
// hand issue.Token to the browser
```

### `IssueResult` / `IssueOptions`

```go
type IssueResult struct {
    OK        bool
    Token     string  // sct_<hex>
    ExpiresIn int     // seconds
    IssuedAt  int64   // unix seconds
    Error     string
    Message   string
}

type IssueOptions struct {
    BindingIP string  // end-user IP for bind_ip enforcement
    TTL       int     // 0 → server default (300)
    MaxUses   int     // 0 → server default
    BindUID   string  // user ID; verify side compares ValidateResult.UID
}
```

### `Client.ModerationCheck(input []ModerationItem, userID string) (*ModerationResult, error)`

Multi-modal content moderation. `input` is a slice of `ModerationItem` —
text and image_url entries can be mixed in one call (OpenAI-compatible).

```go
result, err := client.ModerationCheck([]captchala.ModerationItem{
    captchala.TextItem(userComment),
    captchala.ImageURLItem(uploadedImageURL),
}, userID)

if result.Flagged && result.HasCategory("violence", "csam") {
    // hard block
}
```

### `Client.ModerationText(text, userID string) (*ModerationResult, error)`

Convenience wrapper for plain-text moderation.

```go
result, err := client.ModerationText("user comment here", userID)
```

### `ModerationItem` / `ModerationResult`

```go
type ModerationItem struct {
    Type     string         // "text" or "image_url"
    Text     string         // for type="text"
    ImageURL map[string]any // for type="image_url" — {"url": "https://..."}
}

// Helpers — usually preferred over building items by hand:
captchala.TextItem("...")
captchala.ImageURLItem("https://...")

type ModerationResult struct {
    OK          bool
    Flagged     bool
    Categories  map[string]bool   // category name → tripped
    ContentType string            // "text" / "image" / "mixed"
    Raw         map[string]any    // full upstream payload
    Error       string
    Message     string
}

// Method:
result.HasCategory("violence", "csam")  // true if ANY of the names tripped
```

## Token Types

| Prefix | Source | Security Level |
|--------|--------|----------------|
| `pt_` | Main API | High |
| `offline_` | Backup Service | Medium |
| `client_` | Client-only | Low (cannot verify server-side) |

## Complete Example

```go
package main

import (
    "net/http"
    "os"

    "github.com/gin-gonic/gin"
    captchala "github.com/Captcha-La/captchala-go"
)

var captchaClient *captchala.Client

func init() {
    captchaClient = captchala.NewClient(
        os.Getenv("CAPTCHALA_APP_KEY"),
        os.Getenv("CAPTCHALA_APP_SECRET"),
    )
}

// CaptchaMiddleware validates captcha tokens
func CaptchaMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.PostForm("captcha_token")
        if token == "" {
            token = c.GetHeader("X-Captcha-Token")
        }

        if token == "" {
            c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
                "error": "missing_captcha_token",
            })
            return
        }

        result, _ := captchaClient.Validate(token)
        if !result.Valid {
            c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
                "error":   "captcha_failed",
                "message": result.Error,
            })
            return
        }

        // Pass validation info to handlers
        c.Set("captcha_offline", result.Offline)
        c.Set("captcha_client_only", result.ClientOnly)

        c.Next()
    }
}

func main() {
    r := gin.Default()

    r.POST("/login", CaptchaMiddleware(), func(c *gin.Context) {
        // Captcha passed, handle login
        c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
    })

    r.Run(":8080")
}
```

## Testing

```bash
# Run tests
go test -v

# Integration tests (requires real credentials)
CAPTCHALA_APP_KEY=xxx CAPTCHALA_APP_SECRET=xxx go test -v

# Benchmarks
go test -bench=.
```

## License

MIT
