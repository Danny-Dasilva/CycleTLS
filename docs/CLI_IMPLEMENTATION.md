# CycleTLS CLI Implementation Plan

> **Status: Future Plans** -- This document describes a planned feature that has **not yet been implemented**. The CLI tool described below does not currently exist. This is a design document for future development.

## Overview

Add a curl-like CLI interface to CycleTLS that allows making HTTP requests with TLS fingerprinting directly from the command line.

## User Decisions
- **CLI Style:** curl-like (`cycletls https://url --ja3 "..."`)
- **Binary Name:** `cycletls` with `ctls` symlink
- **Distribution:** Both npm package and standalone releases

---

## Research Summary

### Current Architecture
- CycleTLS Go binary is **exclusively a WebSocket server** (`src/index.go`)
- Core library (`cycletls/`) is **already decoupled** from the server
- Library provides synchronous API: `cycletls.Init().Do(URL, Options, Method)`

### Key Finding: CLI is Feasible Without Breaking Changes

The `cycletls.Do()` method already exists and works independently of the WebSocket server. A CLI would simply be a new entry point that calls this existing API.

## Implementation Approach

### Separate Binary (Recommended)
```
dist/index           ← WebSocket server (existing)
dist/cycletls        ← CLI tool (new)
dist/ctls            ← Symlink to cycletls
```

**Benefits:**
- Zero risk to existing functionality
- Clear separation of concerns
- Users can choose which binary to install

## CLI Design

### Command Structure (curl-like)
```bash
# Basic GET
cycletls https://example.com

# With JA3 fingerprint
cycletls https://example.com --ja3 "771,4865-4867..."

# POST with body
cycletls https://example.com -X POST -d '{"key":"value"}'

# With headers
cycletls https://example.com -H "Authorization: Bearer token"

# With proxy
cycletls https://example.com --proxy socks5://localhost:9050

# Save to file
cycletls https://example.com -o response.html
```

### Flags Mapping (Options struct → CLI flags)

| Options Field | CLI Flag | curl Equivalent |
|---------------|----------|-----------------|
| URL | positional | positional |
| Method | `-X, --request` | `-X` |
| Headers | `-H, --header` | `-H` |
| Body | `-d, --data` | `-d` |
| BodyBytes | `--data-binary` | `--data-binary` |
| Ja3 | `--ja3` | N/A |
| Ja4r | `--ja4r` | N/A |
| UserAgent | `-A, --user-agent` | `-A` |
| Proxy | `-x, --proxy` | `-x` |
| Timeout | `--timeout` | `--connect-timeout` |
| DisableRedirect | `-L, --no-redirect` | (inverse of `-L`) |
| InsecureSkipVerify | `-k, --insecure` | `-k` |
| ForceHTTP1 | `--http1.1` | `--http1.1` |
| ForceHTTP3 | `--http3` | `--http3` |
| ServerName | `--sni` | `--connect-to` |
| Cookies | `-b, --cookie` | `-b` |

### Output Formats
```bash
# Default: body only (like curl)
cycletls https://example.com

# Include headers
cycletls https://example.com -i

# JSON output (structured)
cycletls https://example.com --json

# Verbose (request + response details)
cycletls https://example.com -v
```

## Files to Create/Modify

### New Files
| File | Purpose |
|------|---------|
| `cmd/cycletls/main.go` | CLI entry point |
| `cmd/cycletls/flags.go` | Flag definitions and parsing |
| `cmd/cycletls/output.go` | Response formatting (text, json, headers) |

### Existing Files (No Changes Needed)
| File | Status |
|------|--------|
| `cycletls/index.go` | ✅ Use as-is (Do method) |
| `cycletls/types.go` | ✅ Use as-is (Options, Response) |
| `src/index.go` | ✅ Leave unchanged (server) |

### Build Configuration
```json
// package.json additions
"build:cli:linux": "cd cmd/cycletls && go build -o ../../dist/cycletls",
"build:cli:mac": "cd cmd/cycletls && GOOS=darwin go build -o ../../dist/cycletls-mac",
"build:cli:windows": "cd cmd/cycletls && GOOS=windows go build -o ../../dist/cycletls.exe"
```

## Breaking Changes Assessment

**None required.** The CLI is purely additive:
- Existing TypeScript API unchanged
- Existing WebSocket server unchanged
- New CLI uses existing Go library

## Implementation Phases

### Phase 1: Core CLI
- [ ] Create `cmd/cycletls/main.go` with basic flag parsing
- [ ] Implement GET/POST requests
- [ ] Add --ja3 and --ja4r flags
- [ ] Add header and body support
- [ ] Output response body

### Phase 2: curl Parity
- [ ] Add proxy support (-x)
- [ ] Add timeout support
- [ ] Add redirect control
- [ ] Add cookie support
- [ ] Add output to file (-o)

### Phase 3: Output Formatting
- [ ] Include headers (-i)
- [ ] JSON output (--json)
- [ ] Verbose mode (-v)
- [ ] Error handling and exit codes

### Phase 4: Distribution
- [ ] Build scripts for all platforms
- [ ] Add to npm package
- [ ] Add CLI documentation to README
- [ ] Consider homebrew/apt distribution

## Verification

1. **Basic request works:**
   ```bash
   ./cycletls https://httpbin.org/get
   ```

2. **JA3 fingerprint applied:**
   ```bash
   ./cycletls https://tls.peet.ws/api/all --ja3 "771,..." | jq .tls.ja3_hash
   ```

3. **Headers work:**
   ```bash
   ./cycletls https://httpbin.org/headers -H "X-Custom: value" | jq .headers
   ```

4. **No regressions:**
   - TypeScript tests still pass
   - Go tests still pass
   - WebSocket server still works

## Example Usage (Final Design)

```bash
# Basic GET request
cycletls https://httpbin.org/get
ctls https://httpbin.org/get  # short alias

# With JA3 fingerprint
cycletls https://tls.peet.ws/api/all --ja3 "771,4865-4867-4866..."

# POST with JSON body
cycletls https://api.example.com -X POST \
  -H "Content-Type: application/json" \
  -d '{"key": "value"}'

# With proxy and custom user agent
cycletls https://example.com \
  -x socks5://localhost:9050 \
  -A "Mozilla/5.0 (Windows NT 10.0; Win64; x64)..."

# Save response to file
cycletls https://example.com/image.png -o image.png

# Show headers + body
cycletls https://httpbin.org/headers -i

# JSON output (structured)
cycletls https://httpbin.org/json --json
```

## Distribution Strategy

### npm Package
- Ship CLI binary in `dist/` alongside existing server binaries
- Add `bin` field to package.json for npx support
- Users: `npx cycletls https://example.com`

### Standalone Releases
- GitHub Releases with platform-specific binaries
- Homebrew formula: `brew install cycletls`
- Future: apt/chocolatey packages

## Effort Estimate

| Phase | Scope | Complexity |
|-------|-------|------------|
| Phase 1 | Core CLI | Low - existing Do() API |
| Phase 2 | curl parity | Medium - flag parsing |
| Phase 3 | Output formatting | Low |
| Phase 4 | Distribution | Medium - CI/packaging |

**Total:** ~2-3 days of focused work for a solid MVP
