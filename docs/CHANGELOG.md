# CycleTLS Changelog

## 1.0.21 - (6-20-2022)
### Release Highlights
Json Parse body in response and Cookie examples
### Enhancements
- Return Json body instead of string if response is type Json 🚨 BREAKING CHANGE (users no longer need to `JSON.parse` the response request body)
- Changed basic api to not require CycleTLS `config`
- Add Cookie examples in readme and update Cookie API
- WIP golang release tags and dependabot alerts 
### Bug Fixes
- [Added support for TLS Certificate Compression (causing cloudflare handshake issues)](https://github.com/Danny-Dasilva/CycleTLS/issues/103)
- [Send Correct Frame headers](https://github.com/Danny-Dasilva/CycleTLS/issues/101)
- [Add Temporary support for extension 17](https://github.com/Danny-Dasilva/CycleTLS/issues/100)

## 0.0.20 - (5-16-2022)
### Release Highlights
Add support for extension 17513 and repo cleanup
### Enhancements
- Changed default request timeout to 15 seconds
- [remove duplicate sources](https://github.com/Danny-Dasilva/CycleTLS/issues/85)
- [Added support for extension 17513](https://github.com/Danny-Dasilva/CycleTLS/issues/70)
- add support for `application/pdf`, 
### Bug Fixes
- [properly add grease](https://github.com/Danny-Dasilva/CycleTLS/issues/53)


## 0.0.19 - (4-20-2022)
### Release Highlights
Added image and Arm support
### Enhancements
- [Arm and Arm64 compilation support](https://github.com/Danny-Dasilva/CycleTLS/issues/86)
- [Images are now Base64 encoded for Javascript](https://github.com/Danny-Dasilva/CycleTLS/issues/76)
### Bug Fixes
- [Fix installation dependencies](https://github.com/Danny-Dasilva/CycleTLS/issues/84)
- Don't crash if golang websocket server receives a normal http request
- Update npm dependencies
- Added site support from updates to [fhttp](https://github.com/Danny-Dasilva/fhttp) and [utls](https://github.com/Danny-Dasilva/utls)

## 0.0.18 - (3-26-2022)
### Release Highlights
Swap Golang and TS server
### Enhancements
- Golang is now handles incoming requests, temporary solution till [this](https://github.com/Danny-Dasilva/CycleTLS/issues/55) is addressed 
### Bug Fixes
- [Don't kill process if it does not exist](https://github.com/Danny-Dasilva/CycleTLS/pull/71)

## 0.0.17 - (3-14-2022)
### Release Highlights
Fix fhttp handshake 
### Bug Fixes
- Added utls to the fhttp package and fix imports

## 0.0.16 - (2-15-2022)
### Release Highlights
Fix setting request headers
### Bug Fixes
- [Fix request Headers not being set](https://github.com/Danny-Dasilva/CycleTLS/issues/60)
### Enhancements
- Add request header integration test

## 0.0.15 - (2-11-2022)
### Release Highlights
Support Ordered Request Headers
### Bug Fixes
- Properly form Grease Headers based on UserAgent 
- Refactor error handling 
### Enhancements
- Ordered Request Headers
- [Gzip/brotly/deflate decoding](https://github.com/Danny-Dasilva/CycleTLS/issues/48)
- [Queue requests in case of Process restart](https://github.com/Danny-Dasilva/CycleTLS/issues/50)

## 0.0.14 - (10-06-2021)
### Release Highlights
Bugfix Ja3 Token Parsing and better error handling
### Bug Fixes
- Fix Stack overlflow if token is unsuported [Some Ja3 strings causing stackoverflow](https://github.com/Danny-Dasilva/CycleTLS/issues/51)
- Address Ticket [Spoofed TLS/JA3 fingerprint is detected by some sites](https://github.com/Danny-Dasilva/CycleTLS/issues/47)
- Address Ticket [Set User-Agent for the CONNECT request from the config](https://github.com/Danny-Dasilva/CycleTLS/issues/46)

## 0.0.13 - (09-16-2021)
### Release Highlights
Performance improvements and Better Ja3 Token Parsing 
### Bug Fixes
- Fix Performance issue relating to worker pool [Related Ticket](https://github.com/Danny-Dasilva/CycleTLS/issues/41)
- Address Ticket (fix extension 43) [error: tls: server selected unsupported group](https://github.com/Danny-Dasilva/CycleTLS/issues/43)
- Address Ticket [ fatal error: stack overflow (added support for extensions 22 49 50) ](https://github.com/Danny-Dasilva/CycleTLS/issues/42)
### Enhancements
- Updated Documentation
- Updated Integration tests for new token fixes
- Add new issue template

## 0.0.12 - (08-14-2021)
### Release Highlights
Timeout and Disable redirect integration
### Bug Fixes
- Change to Stable node version V 14 for release
- Address Ticket [Golang cmd window not hiding](https://github.com/Danny-Dasilva/CycleTLS/issues/30)
- Address Ticket [ Method in CycleTLS.Do isn't used ](https://github.com/Danny-Dasilva/CycleTLS/issues/33)
### Enhancements
- Add timeout [ Add a timeout feature ](https://github.com/Danny-Dasilva/CycleTLS/issues/31)
- Optionally disable auto redirect [ Disable auto-redirect ](https://github.com/Danny-Dasilva/CycleTLS/issues/32)
- Add integration tests for timeout and redirect
- Make client exportable [ Make client interface exportable for easy use ](https://github.com/Danny-Dasilva/CycleTLS/pull/37)
- Update CI release with stable node version

## 0.0.11 - (06-6-2021)
### Release Highlights
Memory leak fixes and Clean Exit 
### Bug Fixes
- Address Ticket [Memory Leak on Exit ](https://github.com/Danny-Dasilva/CycleTLS/issues/28) with added unit test
- General Linting on yaml files

### Enhancements
- CI/CD changes for js tests
- New Jest test for clean exit 
- Update ReadMe

## 0.0.10 - (06-4-2021)
### Release Highlights
New Error Handling and bufixes
### Bug Fixes
- Address Ticket [initCycleTLS is not a function](https://github.com/Danny-Dasilva/CycleTLS/issues/26) with added unit test
- Add type hints in TLS transport map

### Enhancements
- New Errors file for checking proxy errors
- Timeout now hard coded to 6 seconds

## 0.0.9 - (05-28-2021)
### Release Highlights
New Jest CI Tests and general bugfixes

### Bug Fixes
- Fix breaking error with file path spaces
- Fix Proxy connection refused and set it to error -> 407
- Fix incorrect proxy url error

### Enhancements
- Add Jest integration tests
- Add Github actions for the above integration test
- Add Cycletls exit api once requests are finished
- CI fixes
- Add Changelog 

## 0.0.8 - (05-24-2021)
### Release Highlights
add fix for crashing js instance

### Bug Fixes
- Fmt or Log printing in golang was causing crashing issues


## 0.0.7 - (05-18-2020)
### Release Highlights
New error logging addition

### Bug Fixes
- Failed timeout requests will now return a 408 instead of crashing the instance, this will help CycleTLS run through Proxy Lists without crashing

