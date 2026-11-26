# Captchala Go SDK

[English](https://github.com/Captcha-La/captchala-go/blob/main/README.md) · [简体中文](https://github.com/Captcha-La/captchala-go/blob/main/README_zh.md) · **日本語** · [한국어](https://github.com/Captcha-La/captchala-go/blob/main/README.ko.md) · [Español](https://github.com/Captcha-La/captchala-go/blob/main/README.es.md) · [Português (BR)](https://github.com/Captcha-La/captchala-go/blob/main/README.pt-BR.md) · [Deutsch](https://github.com/Captcha-La/captchala-go/blob/main/README.de.md)

Captcha トークンをサーバー側で検証するための SDK です。

- [公式サイト](https://captcha.la)
- [ドキュメント](https://docs.captcha.la/sdk/server-go)
- [ダッシュボード](https://dash.captcha.la)

## インストール

```bash
go get github.com/Captcha-La/captchala-go
```

## クイックスタート

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

## API リファレンス

### `NewClient(appKey, appSecret string) *Client`

デフォルトのタイムアウト（5 秒）でクライアントを作成します。

### `NewClientWithTimeout(appKey, appSecret string, timeout time.Duration) *Client`

タイムアウトを指定してクライアントを作成します。

### `Client.Validate(token string) (*ValidateResult, error)`

トークンを検証し、消費します。

### `Client.ValidateWithOptions(token string, keepToken bool) (*ValidateResult, error)`

トークンを検証します。`keepToken` が `true` の場合、トークンは消費されず、再度検証できます。

### `Client.ValidateWithClientIP(token string, keepToken bool, clientIP string) (*ValidateResult, error)`

トークンを検証し、エンドユーザーの IP を送信します。IP は**任意ですが推奨**です。
受信したリクエストからエンドユーザーの IP を渡してください。追加のリスクチェックに
使用されます。省略しても問題ありません（その場合は `Validate` /
`ValidateWithOptions` を使用します）。

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

### `bind_uid` の確認

`bind_uid = "user_42"` を指定して `server_token` を発行した場合は、結果を確認します。

```go
result, err := client.Validate(token)
if err == nil && result.Valid && result.UID != expectedUserID {
    // pass_token was issued for a different user — reject
}
```

### `Client.IssueServerToken(action string) (*IssueResult, error)`

デフォルトのオプション（IP/UID バインドなし、TTL/maxUses はデフォルト値）で
使い捨ての `sct_` server token を発行します。

### `Client.IssueServerTokenWithOptions(action string, opts IssueOptions) (*IssueResult, error)`

すべてのオプションを指定して server token を発行します。返された `Token` は
`serverToken` プロパティとしてブラウザ SDK に渡してください。一度だけ使用でき、
アクションに紐づき、必要に応じて IP/UID にバインドされます。

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

マルチモーダルのコンテンツモデレーションです。`input` は `ModerationItem` のスライスで、
1 回の呼び出しにテキストと image_url の項目を混在させることができます（OpenAI 互換）。

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

プレーンテキストのモデレーション用の簡易ラッパーです。

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

## トークンの種類

| プレフィックス | 発行元 | セキュリティレベル |
|--------|--------|----------------|
| `pt_` | メイン API | 高 |
| `offline_` | バックアップサービス | 中 |
| `dg_` | メイン API、縮退モード（クォータ超過時など） | `Valid` は常に `false`。`Degraded` を確認して判断してください |
| `client_` | クライアント専用のプレースホルダー（ネイティブ SDK） | なし（下記参照） |
| `co_` | クライアント専用のプレースホルダー（Web SDK） | なし（下記参照） |

### クライアント専用トークン（`client_` / `co_`）

これらのプレースホルダーは、メイン API とバックアップサービスの**両方**に到達できなかった
場合にのみデバイス上で生成されます。署名もサーバー側の記録もありません。
v1.2.3 以降、SDK はこれらを無条件には受け入れません。

1. まずメイン API に疎通確認を行います（タイムアウト 2 秒）。何らかの HTTP 応答があれば
   障害は発生していないと判断し、`Valid=false`、`ClientOnly=true`、`Error="client_only_rejected_api_reachable"` となります。
2. そうでなければバックアップ API に疎通確認を行います（タイムアウト 2 秒）。HTTP 応答があれば
   同様に拒否されます（その場合、デバイスは `offline_` トークンを受け取っているはずです）。
3. 両方に到達できない場合（ネットワークエラー / タイムアウト）に限り、`Valid=true`、
   `ClientOnly=true`、`Offline=true` となります。

疎通確認の結果はプロセス内にキャッシュされます（到達可能は 30 秒、到達不可は 5 秒）。
また、同時に行われる検証はエンドポイントごとに 1 回の疎通確認を共有するため、障害時に
すべてのリクエストが 2 回分のタイムアウトを待つことはありません。

機密性の高いアクション（ログイン、決済）では、障害時であってもクライアント専用トークンを
拒否できます。

```go
client.RejectClientOnly = true
// client_ / co_ -> Valid=false, Error=captchala.ErrClientOnlyPolicy ("client_only_rejected_by_policy")
```

これらのプレフィックスを判定するには `captchala.IsClientOnlyToken(token)` を使用します。

## 完全な例

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

## テスト

```bash
# Run tests
go test -v

# Integration tests (requires real credentials)
CAPTCHALA_APP_KEY=xxx CAPTCHALA_APP_SECRET=xxx go test -v

# Benchmarks
go test -bench=.
```

## ライセンス

MIT
