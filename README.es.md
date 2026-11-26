# Captchala Go SDK

[English](https://github.com/Captcha-La/captchala-go/blob/main/README.md) · [简体中文](https://github.com/Captcha-La/captchala-go/blob/main/README_zh.md) · [日本語](https://github.com/Captcha-La/captchala-go/blob/main/README.ja.md) · [한국어](https://github.com/Captcha-La/captchala-go/blob/main/README.ko.md) · **Español** · [Português (BR)](https://github.com/Captcha-La/captchala-go/blob/main/README.pt-BR.md) · [Deutsch](https://github.com/Captcha-La/captchala-go/blob/main/README.de.md)

SDK de servidor para validar tokens de Captcha.

- [Sitio web](https://captcha.la)
- [Documentación](https://docs.captcha.la/sdk/server-go)
- [Panel](https://dash.captcha.la)

## Instalación

```bash
go get github.com/Captcha-La/captchala-go
```

## Inicio rápido

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

## Referencia de la API

### `NewClient(appKey, appSecret string) *Client`

Crea un cliente con el tiempo de espera predeterminado de 5 segundos.

### `NewClientWithTimeout(appKey, appSecret string, timeout time.Duration) *Client`

Crea un cliente con un tiempo de espera personalizado.

### `Client.Validate(token string) (*ValidateResult, error)`

Valida y consume un token.

### `Client.ValidateWithOptions(token string, keepToken bool) (*ValidateResult, error)`

Valida un token. Si `keepToken` es `true`, el token no se consume y puede volver a validarse.

### `Client.ValidateWithClientIP(token string, keepToken bool, clientIP string) (*ValidateResult, error)`

Valida un token y envía la IP del usuario final. La IP es **opcional, pero
recomendable**: pasa la IP del usuario final tomada de la petición entrante; se usa
para comprobaciones de riesgo adicionales. Puedes omitirla sin problema (en ese caso usa
`Validate` / `ValidateWithOptions`).

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

### Verificar `bind_uid`

Si emitiste el `server_token` con `bind_uid = "user_42"`, comprueba el resultado:

```go
result, err := client.Validate(token)
if err == nil && result.Valid && result.UID != expectedUserID {
    // pass_token was issued for a different user — reject
}
```

### `Client.IssueServerToken(action string) (*IssueResult, error)`

Emite un server token `sct_` de un solo uso con las opciones predeterminadas (sin
vinculación a IP/UID, TTL/maxUses predeterminados).

### `Client.IssueServerTokenWithOptions(action string, opts IssueOptions) (*IssueResult, error)`

Emite un server token con control total de las opciones. Pasa el `Token` devuelto al
SDK de navegador mediante la prop `serverToken`: es de un solo uso, está limitado a una
acción y, opcionalmente, vinculado a una IP/UID.

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

Moderación de contenido multimodal. `input` es un slice de `ModerationItem`; en una
misma llamada se pueden mezclar entradas de texto e image_url (compatible con OpenAI).

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

Atajo para moderar texto plano.

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

## Tipos de token

| Prefijo | Origen | Nivel de seguridad |
|--------|--------|----------------|
| `pt_` | API principal | Alto |
| `offline_` | Servicio de respaldo | Medio |
| `dg_` | API principal, modo degradado (p. ej., cuota agotada) | `Valid` siempre es `false`; comprueba `Degraded` y decide tú |
| `client_` | Marcador solo de cliente (SDK nativos) | Ninguno; ver abajo |
| `co_` | Marcador solo de cliente (SDK web) | Ninguno; ver abajo |

### Tokens solo de cliente (`client_` / `co_`)

Estos marcadores se generan en el dispositivo solo cuando **ni** la API principal
**ni** el servicio de respaldo estaban accesibles. No llevan firma ni tienen registro en
el servidor. Desde la v1.2.3, el SDK ya no los acepta a ciegas:

1. Primero sondea la API principal (tiempo de espera de 2 s). Cualquier respuesta HTTP
   significa que no hay caída → `Valid=false`, `ClientOnly=true`, `Error="client_only_rejected_api_reachable"`.
2. Si no, sondea la API de respaldo (tiempo de espera de 2 s). Cualquier respuesta HTTP →
   se rechaza de la misma forma (el dispositivo debería haber recibido un token `offline_`).
3. Solo si ninguna de las dos es accesible (error de red / tiempo de espera agotado) → `Valid=true`,
   `ClientOnly=true`, `Offline=true`.

Los resultados del sondeo se guardan en caché dentro del proceso: "accesible" durante
30 segundos e "inaccesible" durante 5 segundos. Las validaciones concurrentes comparten un
único sondeo por endpoint, así que una caída no obliga a cada petición a esperar dos tiempos
de espera.

Para acciones sensibles (inicio de sesión, pagos) puedes rechazar los tokens solo de
cliente incluso durante una caída:

```go
client.RejectClientOnly = true
// client_ / co_ -> Valid=false, Error=captchala.ErrClientOnlyPolicy ("client_only_rejected_by_policy")
```

Usa `captchala.IsClientOnlyToken(token)` para detectar estos prefijos.

## Ejemplo completo

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

## Pruebas

```bash
# Run tests
go test -v

# Integration tests (requires real credentials)
CAPTCHALA_APP_KEY=xxx CAPTCHALA_APP_SECRET=xxx go test -v

# Benchmarks
go test -bench=.
```

## Licencia

MIT
