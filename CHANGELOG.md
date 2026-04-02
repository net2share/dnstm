# Changelog

## [0.7.1](https://github.com/net2share/dnstm/compare/v0.7.0...v0.7.1) (2026-04-02)


### Bug Fixes

* **vaydns:** validate record-type, show VayDNS details in CLI output ([4ab1b8d](https://github.com/net2share/dnstm/commit/4ab1b8d2f10c361b85887110548b2711370a5f7d))

## [0.7.0](https://github.com/net2share/dnstm/compare/v0.6.8...v0.7.0) (2026-04-02)


### Features

* add VayDNS transport support ([#78](https://github.com/net2share/dnstm/issues/78)) ([dcba892](https://github.com/net2share/dnstm/commit/dcba892d810d3570203638b39e94d2ecb8a7784d))
* improve TUI menus, share output, and update flow ([2d46059](https://github.com/net2share/dnstm/commit/2d46059aba61d13400ad5969fb52009d741f9e93))
* **vaydns:** add full VayDNS config support to TUI and CLI ([7e70d37](https://github.com/net2share/dnstm/commit/7e70d37633447f90e19d7e005731a670b9312d10))
* **vaydns:** add queue-size, kcp-window-size, queue-overflow, log-level ([fae0e84](https://github.com/net2share/dnstm/commit/fae0e8457ed1dea6e338b1296746b580f1401e2d))
* **vaydns:** add record-type support ([0f16054](https://github.com/net2share/dnstm/commit/0f1605412bd44cc0961ef2874bdb11804700d2f5))


### Bug Fixes

* enforce domain uniqueness only in multi mode ([d009f7e](https://github.com/net2share/dnstm/commit/d009f7e7095b6bc301b9626190de33324cb63050))
* **vaydns:** add missing status display, config load, and e2e support ([231be0b](https://github.com/net2share/dnstm/commit/231be0bb337428301aae16ca2f591dd19e833ddd))
* **vaydns:** improve TUI labels and queue-size minimum ([8f6f04b](https://github.com/net2share/dnstm/commit/8f6f04b82b2bf075e19578ac26d7c67fad48f96a))
* **vaydns:** include transport-specific fields in share links ([a366f17](https://github.com/net2share/dnstm/commit/a366f174c74a0a2141d84b0941960a461170950c))
* **vaydns:** integrate with update, install, uninstall, and status systems ([2361b41](https://github.com/net2share/dnstm/commit/2361b4149e027930b9d1f67afcf813d4e9b6fbab))
* **vaydns:** version bump, kebab-case flags, display info, re-exports ([b96de6b](https://github.com/net2share/dnstm/commit/b96de6b5dfe22bc7244167c2c229b4cf38402bdf))

## [0.6.8](https://github.com/net2share/dnstm/compare/v0.6.7...v0.6.8) (2026-03-06)


### Features

* add SOCKS5 authentication for built-in backend ([#70](https://github.com/net2share/dnstm/issues/70)) ([236c30d](https://github.com/net2share/dnstm/commit/236c30d39523f15da4a75b6d203411d487822c6b))
* add tunnel share command with dnst:// URL scheme ([#66](https://github.com/net2share/dnstm/issues/66)) ([f8f8076](https://github.com/net2share/dnstm/commit/f8f8076eda05f8a41485cc8d1b6b6dead9157bcf))
* make MTU configurable in TUI interactive flow ([#69](https://github.com/net2share/dnstm/issues/69)) ([6022421](https://github.com/net2share/dnstm/commit/60224214c8ecb6abffc1e4034b22b0299465fc23))

## [0.6.7](https://github.com/net2share/dnstm/compare/v0.6.6...v0.6.7) (2026-02-26)


### Bug Fixes

* bump sshtun-user to v0.3.5 ([#63](https://github.com/net2share/dnstm/issues/63)) ([500b896](https://github.com/net2share/dnstm/commit/500b8963201f527e83fdb10cef0f372000799662))

## [0.6.6](https://github.com/net2share/dnstm/compare/v0.6.5...v0.6.6) (2026-02-24)


### Features

* add remote e2e test script with named phases ([cf9ec70](https://github.com/net2share/dnstm/commit/cf9ec700cad4b822a4f37080df30c1abc8f27baf))


### Bug Fixes

* pass --force to update when user confirms in install script ([a702ce0](https://github.com/net2share/dnstm/commit/a702ce0b95c80c7df2c129205cfed0eaf82b0a77))

## [0.6.5](https://github.com/net2share/dnstm/compare/v0.6.4...v0.6.5) (2026-02-14)


### Features

* add separators and external tools section to main menu ([#50](https://github.com/net2share/dnstm/issues/50)) ([73bf7fa](https://github.com/net2share/dnstm/commit/73bf7fad94518ee24958b550c86ce7ea2e522f02))
* couple tunnel start/stop with enable/disable ([87d61da](https://github.com/net2share/dnstm/commit/87d61dab274a458ff578a8ab4b2541bda1d8a964))


### Bug Fixes

* read from /dev/tty and auto-install binaries in install script ([fb872ba](https://github.com/net2share/dnstm/commit/fb872ba7e19ac04c102135456aaf0757a909b858))

## [0.6.4](https://github.com/net2share/dnstm/compare/v0.6.3...v0.6.4) (2026-02-07)


### Features

* fix TUI flickering between menu transitions ([5e9af93](https://github.com/net2share/dnstm/commit/5e9af937cd379bead89e5b41a3d95d75cb6b5f46))

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
