# Changelog

## [0.6.3](https://github.com/net2share/dnstm/compare/v0.6.2...v0.6.3) (2026-02-06)


### Features

* add update command with pinned binary versions ([#43](https://github.com/net2share/dnstm/issues/43)) ([0c8fdfd](https://github.com/net2share/dnstm/commit/0c8fdfd690e8aca47e38cd505dec3d14939cf850))


### Bug Fixes

* detect correct nobody group for microsocks service ([69c0e35](https://github.com/net2share/dnstm/commit/69c0e35b0ddade5f14b3b184f3c01f182df4b7e0))

## [0.6.2](https://github.com/net2share/dnstm/compare/v0.6.1...v0.6.2) (2026-02-01)


### Bug Fixes

* improve install UX and fix uninstall/tunnel-add bugs ([49b0955](https://github.com/net2share/dnstm/commit/49b0955d5fac7b180457fd3dee94b73405706f4a))

## [0.6.1](https://github.com/net2share/dnstm/compare/v0.6.0...v0.6.1) (2026-02-01)


### Features

* enhance config load with cleanup and path validation ([98c70d1](https://github.com/net2share/dnstm/commit/98c70d17e0ea7c9467c10fe836f758d1f0c7be0b))

## [0.6.0](https://github.com/net2share/dnstm/compare/v0.5.0...v0.6.0) (2026-02-01)


### ⚠ BREAKING CHANGES

* separate tunnels from backends
* migrate to action-based architecture

### Features

* add TUI progress views with scrolling and centralized binary manager ([d566b78](https://github.com/net2share/dnstm/commit/d566b78c22589d079e3c0bbf85de5d87aa6a1fd5))


### Code Refactoring

* migrate to action-based architecture ([3f43347](https://github.com/net2share/dnstm/commit/3f4334779a6f1ef892f87f934cf242b05d565e00))
* separate tunnels from backends ([dca42e7](https://github.com/net2share/dnstm/commit/dca42e79d2355833c1284629d0d73719ee02ba8f))

## [0.5.0](https://github.com/net2share/dnstm/compare/v0.4.1...v0.5.0) (2026-01-29)


### ⚠ BREAKING CHANGES

* restructure architecture with router, transport instances, and standalone sshtun-user ([#35](https://github.com/net2share/dnstm/issues/35))
* rewrite architecture with router and transport instances ([#33](https://github.com/net2share/dnstm/issues/33))

### Features

* restructure architecture with router, transport instances, and standalone sshtun-user ([#35](https://github.com/net2share/dnstm/issues/35)) ([37e609e](https://github.com/net2share/dnstm/commit/37e609ecc01ef788fa3c9e8eade2da36bfdcc56a))
* rewrite architecture with router and transport instances ([#33](https://github.com/net2share/dnstm/issues/33)) ([dd923bf](https://github.com/net2share/dnstm/commit/dd923bfcfe187004f8a57876f43912fae2394461))

## [0.4.1](https://github.com/net2share/dnstm/compare/v0.4.0...v0.4.1) (2026-01-27)


### Bug Fixes

* prefer glibc microsocks build and restart services on reconfig ([#30](https://github.com/net2share/dnstm/issues/30)) ([f8d8138](https://github.com/net2share/dnstm/commit/f8d8138f19d6d13d0404216953d171cd377cb423))

## [0.4.0](https://github.com/net2share/dnstm/compare/v0.3.0...v0.4.0) (2026-01-27)


### ⚠ BREAKING CHANGES

* add Shadowsocks provider and consolidate user management ([#28](https://github.com/net2share/dnstm/issues/28))

### Features

* add Shadowsocks provider and consolidate user management ([#28](https://github.com/net2share/dnstm/issues/28)) ([fec1b1e](https://github.com/net2share/dnstm/commit/fec1b1e512ea0c2eb4b121cd985975aad2819b39))

## [0.3.0](https://github.com/net2share/dnstm/compare/v0.2.0...v0.3.0) (2026-01-25)


### ⚠ BREAKING CHANGES

* Provider installation no longer automatically sets up SOCKS proxy or SSH tunnel users. These are now managed separately.

### Features

* decouple SOCKS and SSH user management from providers ([#20](https://github.com/net2share/dnstm/issues/20)) ([9ba6a6a](https://github.com/net2share/dnstm/commit/9ba6a6ac2584d5a7f41b7e40e1546386e31ffbf0))

## [0.2.0](https://github.com/net2share/dnstm/compare/v0.1.3...v0.2.0) (2026-01-25)


### ⚠ BREAKING CHANGES

* add Slipstream-Rust and improve SSH user management ([#19](https://github.com/net2share/dnstm/issues/19))

### Features

* add Slipstream-Rust and improve SSH user management ([#19](https://github.com/net2share/dnstm/issues/19)) ([ad5ba5b](https://github.com/net2share/dnstm/commit/ad5ba5b66d9f2217dce92545739601563a0fe531))
* switch dnstt binaries to GitHub releases ([#17](https://github.com/net2share/dnstm/issues/17)) ([b1f407c](https://github.com/net2share/dnstm/commit/b1f407c6d88446ad5404cce9cf1d385743137207))

## [0.1.3](https://github.com/net2share/dnstm/compare/v0.1.2...v0.1.3) (2026-01-24)


### Features

* rewrite CLI with Cobra, huh, and shared go-corelib/tui ([#15](https://github.com/net2share/dnstm/issues/15)) ([84058b4](https://github.com/net2share/dnstm/commit/84058b45849ce83a538d6d0f649eb72aff195ecb))

## [0.1.2](https://github.com/net2share/dnstm/compare/v0.1.1...v0.1.2) (2026-01-24)


### Features

* add CLI commands and integrate sshtun-user module ([#13](https://github.com/net2share/dnstm/issues/13)) ([908c80a](https://github.com/net2share/dnstm/commit/908c80ac41f52bbff67f4ccf3cf6173a5cf2f266))

## [0.1.1](https://github.com/net2share/dnstm/compare/v0.1.0...v0.1.1) (2026-01-22)


### Features

* add uninstall option to completely remove dnstt ([#10](https://github.com/net2share/dnstm/issues/10)) ([ed46431](https://github.com/net2share/dnstm/commit/ed46431793c21bc7045c1a405c71e0b861f189c9))

## 0.1.0 (2026-01-22)


### Features

* rewrite dnstm in Go ([701d3e1](https://github.com/net2share/dnstm/commit/701d3e105a3200fb16f8f9db2eeea45ac564b24d))
