# CycleTLS Remaining Features & Issues

Based on open GitHub issues as of August 2024. These represent potential features and fixes for future releases.

## High Priority Issues

### Version Compatibility & Stability
- [ ] **#381** - Fix "Can't run on version 2.0.0.0" compatibility issues
- [ ] **#373** - Resolve program freezing and timeout not working properly
- [ ] **#372** - Fix unexpected behavior when multiple NodeJS file instances are running

### Protocol & Feature Enhancements
- [ ] **#380** - Enhance support for binary format handling
- [ ] **#378** - Add HTTP protocol version information in response objects
- [ ] **#375** - Improve extension support for unsupported TLS extensions

## Medium Priority Issues

### Error Handling & Networking
- [ ] **#369** - Return proper errors for "dial tcp: no such host" scenarios
- [ ] **#366** - Improve general error processing for requests

### Advanced Features
- [ ] **#368** - Implement automatic CookieJar covering complex cookie scenarios
- [ ] **#377** - Add manual SIGINT/SIGTERM processing capabilities

## Security & Maintenance
- [ ] **#376** - Address vulnerability in crypto implementation

## Incomplete/Unclear Issues
- [ ] **#364** - "Cycle" (issue needs clarification)

## Implementation Notes

### Binary Format Support (#380)
- Consider extending the existing binary response support from 2.0.0
- May need streaming optimizations for large binary payloads

### HTTP Protocol Version in Response (#378)
- Add protocol version field to response object
- Should indicate whether HTTP/1.1, HTTP/2, or HTTP/3 was used

### Advanced CookieJar (#368)
- Build upon existing cookie support
- Handle complex scenarios like domain/path matching, secure cookies, SameSite attributes
- Consider jar persistence and session management

### Extension Support (#375)
- Extend the current TLS extension handling
- Add support for newer TLS extensions as they become available
- Graceful fallback for unsupported extensions

## Next Steps
1. Prioritize version compatibility issues (#381, #373, #372) for immediate 2.0.x patches
2. Plan binary format and protocol version enhancements for 2.1.0
3. Consider advanced features like enhanced CookieJar for 2.2.0
4. Address security vulnerabilities as hotfixes when identified