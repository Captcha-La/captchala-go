# Captchala Go SDK

[English](https://github.com/Captcha-La/captchala-go/blob/main/README.md) · [简体中文](https://github.com/Captcha-La/captchala-go/blob/main/README_zh.md) · [日本語](https://github.com/Captcha-La/captchala-go/blob/main/README.ja.md) · [한국어](https://github.com/Captcha-La/captchala-go/blob/main/README.ko.md) · [Español](https://github.com/Captcha-La/captchala-go/blob/main/README.es.md) · **Português (BR)** · [Deutsch](https://github.com/Captcha-La/captchala-go/blob/main/README.de.md)

SDK de servidor para validar tokens do Captcha.

- [Site](https://captcha.la)
- [Documentação](https://docs.captcha.la/sdk/server-go)
- [Painel](https://dash.captcha.la)

## Instalação

```bash
go get github.com/Captcha-La/captchala-go
```

## Início rápido

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

## Referência da API

### `NewClient(appKey, appSecret string) *Client`

Cria um cliente com o timeout padrão de 5 segundos.

### `NewClientWithTimeout(appKey, appSecret string, timeout time.Duration) *Client`

Cria um cliente com timeout personalizado.

### `Client.Validate(token string) (*ValidateResult, error)`

Valida e consome um token.

### `Client.ValidateWithOptions(token string, keepToken bool) (*ValidateResult, error)`

Valida um token. Se `keepToken` for `true`, o token não é consumido e pode ser validado novamente.

### `Client.ValidateWithClientIP(token string, keepToken bool, clientIP string) (*ValidateResult, error)`

Valida um token e repassa o IP do usuário final. O IP é **opcional, mas
recomendado**: passe o IP do usuário final obtido da requisição recebida; ele é usado
em verificações de risco adicionais. Pode ser omitido sem problemas (nesse caso, use
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

### Verificando o `bind_uid`

Se você emitiu o `server_token` com `bind_uid = "user_42"`, confira o resultado:

```go
result, err := client.Validate(token)
if err == nil && result.Valid && result.UID != expectedUserID {
    // pass_token was issued for a different user — reject
}
```

### `Client.IssueServerToken(action string) (*IssueResult, error)`

Emite um server token `sct_` de uso único com as opções padrão (sem vínculo de
IP/UID, TTL/maxUses padrão).

### `Client.IssueServerTokenWithOptions(action string, opts IssueOptions) (*IssueResult, error)`

Emite um server token com controle total das opções. Passe o `Token` retornado para o
SDK do navegador pela prop `serverToken`: ele é de uso único, restrito a uma ação e,
opcionalmente, vinculado a IP/UID.

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

Moderação de conteúdo multimodal. `input` é um slice de `ModerationItem`; entradas de
texto e image_url podem ser misturadas em uma única chamada (compatível com OpenAI).

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

Atalho para moderação de texto simples.

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

| Prefixo | Origem | Nível de segurança |
|--------|--------|----------------|
| `pt_` | API principal | Alto |
| `offline_` | Serviço de backup | Médio |
| `dg_` | API principal, modo degradado (ex.: cota esgotada) | `Valid` é sempre `false`; verifique `Degraded` e decida por conta própria |
| `client_` | Placeholder somente do cliente (SDKs nativos) | Nenhum; veja abaixo |
| `co_` | Placeholder somente do cliente (SDK web) | Nenhum; veja abaixo |

### Tokens somente do cliente (`client_` / `co_`)

Esses placeholders são gerados no dispositivo somente quando **tanto** a API principal
**quanto** o serviço de backup estavam inacessíveis. Eles não têm assinatura nem registro
no servidor. Desde a v1.2.3, o SDK não os aceita mais cegamente:

1. Primeiro ele sonda a API principal (timeout de 2 s). Qualquer resposta HTTP significa
   que não há indisponibilidade → `Valid=false`, `ClientOnly=true`, `Error="client_only_rejected_api_reachable"`.
2. Caso contrário, sonda a API de backup (timeout de 2 s). Qualquer resposta HTTP → rejeitado
   da mesma forma (o dispositivo deveria ter recebido um token `offline_`).
3. Somente se ambas estiverem inacessíveis (erro de rede / timeout) → `Valid=true`,
   `ClientOnly=true`, `Offline=true`.

Os resultados da sondagem ficam em cache no processo: "acessível" por 30 segundos e
"inacessível" por 5 segundos. Validações concorrentes compartilham uma única sondagem por
endpoint, então uma indisponibilidade não faz toda requisição esperar dois timeouts.

Para ações sensíveis (login, pagamento), você pode recusar tokens somente do cliente
mesmo durante uma indisponibilidade:

```go
client.RejectClientOnly = true
// client_ / co_ -> Valid=false, Error=captchala.ErrClientOnlyPolicy ("client_only_rejected_by_policy")
```

Use `captchala.IsClientOnlyToken(token)` para detectar esses prefixos.

## Exemplo completo

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

## Testes

```bash
# Run tests
go test -v

# Integration tests (requires real credentials)
CAPTCHALA_APP_KEY=xxx CAPTCHALA_APP_SECRET=xxx go test -v

# Benchmarks
go test -bench=.
```

## Licença

MIT
