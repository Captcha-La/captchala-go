# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [1.2.3] - 2026-10-27

### Security
- Client-only tokens (`client_`, and now also the web SDK's `co_`) are no
  longer accepted unconditionally. `Validate*` probes the main API and then the
  backup API (2s timeout each); if either answers with any HTTP response the
  token is rejected with `Valid=false`, `ClientOnly=true`,
  `Error="client_only_rejected_api_reachable"`. The token is accepted
  (`Valid=true`, `ClientOnly=true`) only when both endpoints are unreachable.
  Positive reachability results are cached in-process for 30 seconds.
  Previously any string starting with `client_` passed without a network call.

### Added
- `PrefixClientWeb` (`co_`), `PrefixDegraded` (`dg_`), `ErrClientOnlyRejected`
  constants and `IsClientOnlyToken(token)` helper.

### Fixed
- Non-JSON or non-2xx responses (e.g. an HTML 502 from a proxy) no longer make
  `Validate*` return a `nil` result; a result with `Error="http_<status>"` or
  `"invalid_response"` is returned alongside the error. A `code:0` body on a
  non-2xx status is never treated as success.
- Response bodies are read through a 1 MiB `io.LimitReader`.
- A `Client` built as a struct literal (without `NewClient`) no longer panics
  on a nil HTTP client.

## [1.2.2] - 2026-06-20

### Added
- `ValidateResult.CaptchaArgs` (`CaptchaArgs` struct) — solve-context echo
  returned by the dashboard: `Platform`, `UserIP`, `Referer` (web page URL),
  `Pkg` (native app id), `SolvedAt` (unix seconds), `RiskScore` (0-100). All
  informational.

### Changed
- `ValidateWithClientIP`'s IP argument is now **optional but recommended** —
  pass the end-user IP if you have it (used for additional risk checks), or use
  `Validate` / `ValidateWithOptions`. The IP is no longer used for an exact
  solve-vs-submit comparison (which rejected legitimate users under CDN +
  dual-stack IPv4/IPv6). Backward compatible.

## [1.0.2] - Unreleased

### Added
- `Client.IssueServerToken(action) (*IssueResult, error)` and
  `Client.IssueServerTokenWithOptions(action, IssueOptions) (*IssueResult, error)` —
  mint a one-time `sct_` server token via `POST /v1/server/challenge/issue`.
  Hand the returned token to the browser SDK as the `serverToken` prop for the
  recommended production flow (single-use, action-scoped, optionally IP/UID-bound).
- `Client.ModerationCheck(input []ModerationItem, userID) (*ModerationResult, error)` —
  multi-modal content moderation (`POST /v1/moderation/check`). `input` mixes
  text and `image_url` items in OpenAI-compatible format. Helper constructors
  `TextItem(text)` and `ImageURLItem(url)` provided.
- `Client.ModerationText(text, userID) (*ModerationResult, error)` —
  convenience wrapper for plain-text moderation (`POST /v1/moderation/text`).
- New result types `IssueResult`, `IssueOptions`, `ModerationResult`,
  `ModerationItem` mirror the `ValidateResult` shape — explicit fields,
  `HasCategory(...names)` helper on `ModerationResult`.
- Test coverage: `httptest`-backed happy path + omitted-optional-fields +
  upstream-error propagation for all three new methods (8 new tests, 21 total).

### Compatibility
- No breaking changes. New methods/types are additive; existing `Validate*`
  call sites are unaffected. Three new optional URL-override fields on
  `Client` (`IssueURL`, `ModerationCheckURLStr`, `ModerationTextURLStr`)
  default to empty (=use production endpoints).

## [0.2.0] - Unreleased

### Added
- `Client.ValidateWithClientIP(token, keepToken, clientIP)` — forwards the
  end-user IP to the backend so that tokens issued with `bind_ip` can be
  verified. When `clientIP` is empty the field is omitted and the call is
  equivalent to `ValidateWithOptions`.
- `ValidateResult.UID` — populated with the `uid` returned by the backend
  when the `pass_token` was issued against a `server_token` with `bind_uid`.
  Integrators can compare this against the expected user ID to verify the
  captcha was solved for the intended account.
- Test coverage: `httptest`-backed happy path, client_ip body serialisation,
  uid parsing, offline backup URL routing, network-error handling, and
  upstream error propagation.

### Changed
- Internal refactor: `Validate`, `ValidateWithOptions` and
  `ValidateWithClientIP` all funnel through a shared `validateInternal`
  helper. Public function signatures are unchanged.
- Added `BaseURL` / `BackupURL` fields to `Client` to enable testing against
  mock servers. When left empty they fall back to the public endpoints —
  no behavioural change for existing users.

### Compatibility
- No breaking changes. All pre-0.2.0 call sites continue to compile and
  behave identically.

## [0.1.0]

- Initial release.
