# Changelog

Breaking changes to exported identifiers, signatures or behaviour, for the
applications that import MCPFusion packages.

## Unreleased

- Removed `global.AppName` and `global.AppVersion`. The application identity
  now lives in the `app` package; use `app.Name()`, `app.Version()` (display)
  or `app.SemVer()` (protocol handshakes and comparisons).
