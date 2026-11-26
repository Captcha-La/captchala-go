# Captchala Go SDK

[English](https://github.com/Captcha-La/captchala-go/blob/main/README.md) · [简体中文](https://github.com/Captcha-La/captchala-go/blob/main/README_zh.md) · [日本語](https://github.com/Captcha-La/captchala-go/blob/main/README.ja.md) · **한국어** · [Español](https://github.com/Captcha-La/captchala-go/blob/main/README.es.md) · [Português (BR)](https://github.com/Captcha-La/captchala-go/blob/main/README.pt-BR.md) · [Deutsch](https://github.com/Captcha-La/captchala-go/blob/main/README.de.md)

서버에서 Captcha 토큰을 검증하기 위한 SDK입니다.

- [웹사이트](https://captcha.la)
- [문서](https://docs.captcha.la/sdk/server-go)
- [대시보드](https://dash.captcha.la)

## 설치

```bash
go get github.com/Captcha-La/captchala-go
```

## 빠른 시작

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

## API 레퍼런스

### `NewClient(appKey, appSecret string) *Client`

기본 타임아웃(5초)으로 클라이언트를 생성합니다.

### `NewClientWithTimeout(appKey, appSecret string, timeout time.Duration) *Client`

타임아웃을 지정하여 클라이언트를 생성합니다.

### `Client.Validate(token string) (*ValidateResult, error)`

토큰을 검증하고 소비합니다.

### `Client.ValidateWithOptions(token string, keepToken bool) (*ValidateResult, error)`

토큰을 검증합니다. `keepToken`이 `true`이면 토큰이 소비되지 않아 다시 검증할 수 있습니다.

### `Client.ValidateWithClientIP(token string, keepToken bool, clientIP string) (*ValidateResult, error)`

토큰을 검증하고 최종 사용자의 IP를 함께 전달합니다. IP는 **선택 사항이지만 권장**됩니다.
수신한 요청에서 최종 사용자의 IP를 전달하면 추가 위험 검사에 사용됩니다.
생략해도 됩니다(이 경우 `Validate` / `ValidateWithOptions`를 사용하세요).

### `ValidateResult`

```go
type ValidateResult struct {
    Valid       bool        // Whether validation passed
    Offline     bool        // Whether this was offline verification
    ClientOnly  bool        // Whether this is a client-only token
    ChallengeID string      // Challenge ID
    Action      string      // Business action
    UID         string      // User ID bound via bind_uid (for server-side identity check)
    Error       string      // Error message
    Warning     string      // Warning message
    CaptchaArgs CaptchaArgs // Solve-context echo (informational only)
}

// CaptchaArgs is the solve-time context, echoed back on validate
// (Geetest-style). All fields are informational — NEVER gate pass/fail on them.
type CaptchaArgs struct {
    Platform  string // web / android / ios / flutter / windows / ...
    UserIP    string // end-user IP recorded at solve time
    Referer   string // web: solve page URL (empty on native)
    Pkg       string // native: app package / bundle id (empty on web)
    SolvedAt  int64  // solve completion time (unix seconds)
    RiskScore int64  // 0-100, higher = riskier (reCAPTCHA v3 score style)
}
```

```go
result, _ := client.Validate(token)
if result.Valid {
    ip := result.CaptchaArgs.UserIP // solve-time IP (informational)
    _ = ip
}
```

### `bind_uid` 확인

`bind_uid = "user_42"`로 `server_token`을 발급했다면 결과를 확인하세요.

```go
result, err := client.Validate(token)
if err == nil && result.Valid && result.UID != expectedUserID {
    // pass_token was issued for a different user — reject
}
```

### `Client.IssueServerToken(action string) (*IssueResult, error)`

기본 옵션(IP/UID 바인딩 없음, 기본 TTL/maxUses)으로 일회용 `sct_` server token을
발급합니다.

### `Client.IssueServerTokenWithOptions(action string, opts IssueOptions) (*IssueResult, error)`

모든 옵션을 직접 지정하여 server token을 발급합니다. 반환된 `Token`을
`serverToken` prop으로 브라우저 SDK에 전달하세요. 한 번만 사용할 수 있고,
액션 단위로 제한되며, 필요에 따라 IP/UID에 바인딩됩니다.

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

멀티모달 콘텐츠 모더레이션입니다. `input`은 `ModerationItem`의 슬라이스이며,
한 번의 호출에 텍스트와 image_url 항목을 섞어 보낼 수 있습니다(OpenAI 호환).

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

일반 텍스트 모더레이션을 위한 간편 래퍼입니다.

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

## 토큰 유형

| 접두사 | 출처 | 보안 수준 |
|--------|--------|----------------|
| `pt_` | 메인 API | 높음 |
| `offline_` | 백업 서비스 | 중간 |
| `dg_` | 메인 API, 성능 저하 모드(예: 할당량 소진) | `Valid`는 항상 `false`입니다. `Degraded`를 확인하고 직접 판단하세요 |
| `client_` | 클라이언트 전용 플레이스홀더(네이티브 SDK) | 없음 — 아래 참고 |
| `co_` | 클라이언트 전용 플레이스홀더(웹 SDK) | 없음 — 아래 참고 |

### 클라이언트 전용 토큰(`client_` / `co_`)

이 플레이스홀더는 메인 API와 백업 서비스 **모두**에 연결할 수 없을 때만 기기에서
생성됩니다. 서명도 없고 서버 측 기록도 없습니다. v1.2.3부터 SDK는 이를 무조건
수락하지 않습니다.

1. 먼저 메인 API를 확인합니다(타임아웃 2초). HTTP 응답이 하나라도 오면 장애가 없는
   것이므로 `Valid=false`, `ClientOnly=true`, `Error="client_only_rejected_api_reachable"`가 됩니다.
2. 그렇지 않으면 백업 API를 확인합니다(타임아웃 2초). HTTP 응답이 오면 같은 방식으로
   거부됩니다(이 경우 기기는 `offline_` 토큰을 받았어야 합니다).
3. 둘 다 연결할 수 없을 때(네트워크 오류 / 타임아웃)에만 `Valid=true`,
   `ClientOnly=true`, `Offline=true`가 됩니다.

확인 결과는 프로세스 내에 캐시됩니다. "연결 가능"은 30초, "연결 불가"는 5초 동안
유지되며, 동시에 진행되는 검증은 엔드포인트마다 하나의 확인 요청을 공유하므로
장애 중에도 모든 요청이 타임아웃을 두 번씩 기다리지 않습니다.

민감한 작업(로그인, 결제)에서는 장애 중에도 클라이언트 전용 토큰을 거부할 수 있습니다.

```go
client.RejectClientOnly = true
// client_ / co_ -> Valid=false, Error=captchala.ErrClientOnlyPolicy ("client_only_rejected_by_policy")
```

이러한 접두사를 감지하려면 `captchala.IsClientOnlyToken(token)`을 사용하세요.

## 전체 예제

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

## 테스트

```bash
# Run tests
go test -v

# Integration tests (requires real credentials)
CAPTCHALA_APP_KEY=xxx CAPTCHALA_APP_SECRET=xxx go test -v

# Benchmarks
go test -bench=.
```

## 라이선스

MIT
