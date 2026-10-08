# Changelog

Breaking changes to exported identifiers, signatures or behaviour, for the
applications that import MCPFusion packages.

## Unreleased

- Removed `global.AppName` and `global.AppVersion`. The application identity
  now lives in the `app` package; use `app.Name()`, `app.Version()` (display)
  or `app.SemVer()` (protocol handshakes and comparisons).
- `-debug` now defaults to false. Deployments that relied on debug logging
  must pass `-debug` (for example in `mcpfusion.service`).
- The error returned by `OAuth2ExternalStrategy.Authenticate` when no token is
  stored no longer ends with a period. Callers that match on the exact text
  must be updated.
