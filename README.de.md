# Captchala Go SDK

[English](https://github.com/Captcha-La/captchala-go/blob/main/README.md) · [简体中文](https://github.com/Captcha-La/captchala-go/blob/main/README_zh.md) · [日本語](https://github.com/Captcha-La/captchala-go/blob/main/README.ja.md) · [한국어](https://github.com/Captcha-La/captchala-go/blob/main/README.ko.md) · [Español](https://github.com/Captcha-La/captchala-go/blob/main/README.es.md) · [Português (BR)](https://github.com/Captcha-La/captchala-go/blob/main/README.pt-BR.md) · **Deutsch**

Serverseitiges SDK zur Validierung von Captcha-Tokens.

- [Website](https://captcha.la)
- [Dokumentation](https://docs.captcha.la/sdk/server-go)
- [Dashboard](https://dash.captcha.la)

## Installation

```bash
go get github.com/Captcha-La/captchala-go
```

## Schnellstart

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

## API-Referenz

### `NewClient(appKey, appSecret string) *Client`

Erstellt einen Client mit dem Standard-Timeout von 5 Sekunden.

### `NewClientWithTimeout(appKey, appSecret string, timeout time.Duration) *Client`

Erstellt einen Client mit eigenem Timeout.

### `Client.Validate(token string) (*ValidateResult, error)`

Validiert einen Token und verbraucht ihn.

### `Client.ValidateWithOptions(token string, keepToken bool) (*ValidateResult, error)`

Validiert einen Token. Ist `keepToken` gleich `true`, wird der Token nicht verbraucht und kann erneut validiert werden.

### `Client.ValidateWithClientIP(token string, keepToken bool, clientIP string) (*ValidateResult, error)`

Validiert einen Token und übermittelt die IP des Endnutzers. Die IP ist **optional,
aber empfohlen**: Übergib die IP des Endnutzers aus der eingehenden Anfrage; sie wird
für zusätzliche Risikoprüfungen verwendet. Sie kann problemlos weggelassen werden (dann
`Validate` / `ValidateWithOptions` verwenden).

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

### `bind_uid` prüfen

Wenn du den `server_token` mit `bind_uid = "user_42"` ausgestellt hast, prüfe das Ergebnis:

```go
result, err := client.Validate(token)
if err == nil && result.Valid && result.UID != expectedUserID {
    // pass_token was issued for a different user — reject
}
```

### `Client.IssueServerToken(action string) (*IssueResult, error)`

Stellt einen einmalig verwendbaren `sct_`-Server-Token mit Standardoptionen aus (keine
IP/UID-Bindung, Standardwerte für TTL/maxUses).

### `Client.IssueServerTokenWithOptions(action string, opts IssueOptions) (*IssueResult, error)`

Stellt einen Server-Token mit voller Kontrolle über die Optionen aus. Übergib den
zurückgegebenen `Token` als `serverToken`-Prop an das Browser-SDK: einmalig verwendbar,
an eine Aktion gebunden und optional an IP/UID gebunden.

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

Multimodale Inhaltsmoderation. `input` ist ein Slice von `ModerationItem`; Text- und
image_url-Einträge lassen sich in einem Aufruf mischen (OpenAI-kompatibel).

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

Komfort-Wrapper für die Moderation von reinem Text.

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

## Token-Typen

| Präfix | Quelle | Sicherheitsstufe |
|--------|--------|----------------|
| `pt_` | Haupt-API | Hoch |
| `offline_` | Backup-Dienst | Mittel |
| `dg_` | Haupt-API, eingeschränkter Modus (z. B. Kontingent erschöpft) | `Valid` ist immer `false`; prüfe `Degraded` und entscheide selbst |
| `client_` | Reiner Client-Platzhalter (native SDKs) | Keine – siehe unten |
| `co_` | Reiner Client-Platzhalter (Web-SDK) | Keine – siehe unten |

### Reine Client-Tokens (`client_` / `co_`)

Diese Platzhalter werden nur dann auf dem Gerät erzeugt, wenn **weder** die Haupt-API
**noch** der Backup-Dienst erreichbar waren. Sie haben keine Signatur und keinen
Eintrag auf dem Server. Seit v1.2.3 akzeptiert das SDK sie nicht mehr ungeprüft:

1. Zuerst wird die Haupt-API geprüft (2 s Timeout). Jede HTTP-Antwort bedeutet, dass kein
   Ausfall vorliegt → `Valid=false`, `ClientOnly=true`, `Error="client_only_rejected_api_reachable"`.
2. Andernfalls wird die Backup-API geprüft (2 s Timeout). Jede HTTP-Antwort → Ablehnung
   auf die gleiche Weise (das Gerät hätte einen `offline_`-Token erhalten müssen).
3. Nur wenn beide nicht erreichbar sind (Netzwerkfehler / Timeout) → `Valid=true`,
   `ClientOnly=true`, `Offline=true`.

Die Prüfergebnisse werden im Prozess zwischengespeichert: "erreichbar" für 30 Sekunden,
"nicht erreichbar" für 5 Sekunden. Gleichzeitige Validierungen teilen sich eine Prüfung
pro Endpunkt, sodass bei einem Ausfall nicht jede Anfrage zwei Timeouts abwarten muss.

Für sensible Aktionen (Login, Zahlung) kannst du reine Client-Tokens auch während eines
Ausfalls ablehnen:

```go
client.RejectClientOnly = true
// client_ / co_ -> Valid=false, Error=captchala.ErrClientOnlyPolicy ("client_only_rejected_by_policy")
```

Mit `captchala.IsClientOnlyToken(token)` erkennst du diese Präfixe.

## Vollständiges Beispiel

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

## Tests

```bash
# Run tests
go test -v

# Integration tests (requires real credentials)
CAPTCHALA_APP_KEY=xxx CAPTCHALA_APP_SECRET=xxx go test -v

# Benchmarks
go test -bench=.
```

## Lizenz

MIT
