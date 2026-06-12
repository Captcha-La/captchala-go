# Captchala Go SDK

服务端验证 Captcha Token 的 Go SDK。

[English](README.md) | **简体中文**

- [官网](https://captcha.la)
- [文档](https://docs.captcha.la/zh-CN/sdk/server-go)
- [Dashboard](https://dash.captcha.la)

## 安装

```bash
go get github.com/Captcha-La/captchala-go
```

## 快速开始

```go
package main

import (
    "fmt"
    "log"

    captchala "github.com/Captcha-La/captchala-go"
)

func main() {
    // 创建客户端
    client := captchala.NewClient("your_app_key", "your_app_secret")

    // 验证 Token
    result, err := client.Validate(token)
    if err != nil {
        log.Printf("验证错误: %v", err)
    }

    if result.Valid {
        fmt.Println("验证通过")
        if result.Offline {
            fmt.Println("这是离线验证")
        }
    } else {
        fmt.Printf("验证失败: %s\n", result.Error)
    }
}
```

## API

### `NewClient(appKey, appSecret string) *Client`

创建客户端实例（默认 5 秒超时）。

### `NewClientWithTimeout(appKey, appSecret string, timeout time.Duration) *Client`

创建客户端实例（自定义超时时间）。

### `Client.Validate(token string) (*ValidateResult, error)`

验证 Token（消费 Token）。

### `Client.ValidateWithOptions(token string, keepToken bool) (*ValidateResult, error)`

验证 Token。如果 `keepToken` 为 `true`，Token 不会被消费，可以重复验证。

### `Client.ValidateWithClientIP(token string, keepToken bool, clientIP string) (*ValidateResult, error)`

校验 Token, 同时把终端用户 IP 透传给后端做 `bind_ip` 校验。如果原 `pass_token`
签发时绑定了 IP, 后端会比对; 不匹配会拒绝。传你入站请求里取到的真实用户 IP
（如 `X-Forwarded-For`），不是中间代理 IP。`clientIP` 为空时该字段被省略,
行为与 `ValidateWithOptions` 一致。

```go
result, err := client.ValidateWithClientIP(token, false, userIP)
```

### `ValidateResult` 结构

```go
type ValidateResult struct {
    Valid       bool   // 验证是否通过
    Offline     bool   // 是否为离线验证
    ClientOnly  bool   // 是否为纯客户端 Token
    ChallengeID string // 挑战 ID
    Action      string // 业务动作
    UID         string // server_token 签发时绑定的 user ID, 用于核对身份
    Error       string // 错误信息
    Warning     string // 警告信息
}
```

### 校验 `bind_uid`

如果签发 `server_token` 时带了 `bind_uid = "user_42"`, 校验时核对：

```go
result, err := client.Validate(token)
if err == nil && result.Valid && result.UID != expectedUserID {
    // pass_token 是给别的用户签的 — 拒绝
}
```

### `Client.IssueServerToken(action string) (*IssueResult, error)`

用默认参数签发一次性 `sct_` server token（无 IP/UID 绑定, 用服务端默认 TTL/maxUses）。

### `Client.IssueServerTokenWithOptions(action string, opts IssueOptions) (*IssueResult, error)`

带完整选项签发 server token。返回的 `Token` 下发给前端, 浏览器作为
`serverToken` prop 传给组件 — 单次消费, 绑定 action, 可选绑定 IP / UID。

```go
issue, err := client.IssueServerTokenWithOptions("login", captchala.IssueOptions{
    BindingIP: userIP,    // 不同 IP 来兑换会被后端拒绝
    TTL:       300,       // 秒
    MaxUses:   5,         // SDK 重试预算
    BindUID:   userID,    // 配合 ValidateResult.UID 校验
})
if err != nil || !issue.OK {
    http.Error(w, issue.Error, http.StatusBadRequest)
    return
}
// 把 issue.Token 下发给浏览器
```

### `IssueResult` / `IssueOptions`

```go
type IssueResult struct {
    OK        bool
    Token     string  // sct_<hex>
    ExpiresIn int     // 秒
    IssuedAt  int64   // unix 秒
    Error     string
    Message   string
}

type IssueOptions struct {
    BindingIP string  // 终端用户 IP, 用于 bind_ip 校验
    TTL       int     // 0 → 服务端默认 (300)
    MaxUses   int     // 0 → 服务端默认
    BindUID   string  // user ID; 校验侧比对 ValidateResult.UID
}
```

### `Client.ModerationCheck(input []ModerationItem, userID string) (*ModerationResult, error)`

多模态内容审核。`input` 是 `ModerationItem` 切片 — 文本和 image_url 可以在一次
请求里混合（OpenAI 兼容）。

```go
result, err := client.ModerationCheck([]captchala.ModerationItem{
    captchala.TextItem(userComment),
    captchala.ImageURLItem(uploadedImageURL),
}, userID)

if result.Flagged && result.HasCategory("violence", "csam") {
    // 硬阻断
}
```

### `Client.ModerationText(text, userID string) (*ModerationResult, error)`

纯文本审核快捷方式。

```go
result, err := client.ModerationText("user comment here", userID)
```

### `ModerationItem` / `ModerationResult`

```go
type ModerationItem struct {
    Type     string         // "text" 或 "image_url"
    Text     string         // type="text" 时使用
    ImageURL map[string]any // type="image_url" 时使用 — {"url": "https://..."}
}

// 辅助函数 — 通常优先用而不是手写 item:
captchala.TextItem("...")
captchala.ImageURLItem("https://...")

type ModerationResult struct {
    OK          bool
    Flagged     bool
    Categories  map[string]bool   // category 名 → 是否命中
    ContentType string            // "text" / "image" / "mixed"
    Raw         map[string]any    // 完整上游 payload
    Error       string
    Message     string
}

// 方法:
result.HasCategory("violence", "csam")  // 任一命中即 true
```

## Token 类型

| 前缀 | 来源 | 安全级别 |
|------|------|---------|
| `pt_` | 主服务 | 高 |
| `offline_` | 备用服务 | 中 |
| `client_` | 纯客户端 | 低（无法服务端验证） |

## 完整示例

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

// CaptchaMiddleware 验证码验证中间件
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

        // 将验证结果传递给后续处理
        c.Set("captcha_offline", result.Offline)
        c.Set("captcha_client_only", result.ClientOnly)

        c.Next()
    }
}

func main() {
    r := gin.Default()

    r.POST("/login", CaptchaMiddleware(), func(c *gin.Context) {
        // 验证码已通过，处理登录逻辑
        c.JSON(http.StatusOK, gin.H{"message": "登录成功"})
    })

    r.Run(":8080")
}
```

## 测试

```bash
# 运行测试
go test -v

# 集成测试（需要真实凭证）
CAPTCHALA_APP_KEY=xxx CAPTCHALA_APP_SECRET=xxx go test -v

# 性能测试
go test -bench=.
```

## License

MIT
