# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

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
