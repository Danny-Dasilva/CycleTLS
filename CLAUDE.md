# CycleTLS Development Guide

## Build Commands
- Build TypeScript: `npm run build`
- Build Go binaries: `npm run build:go`
- Build for specific platforms: `npm run build:go:linux:amd64`, `npm run build:go:mac:amd64`, etc.
- Run tests: `npm test` or `jest --detectOpenHandles`
- Run single test: `jest tests/specific.test.ts`
- Full package preparation: `npm run prepare`

## Code Style
### TypeScript
- Use 2-space indentation
- camelCase for variables and functions
- PascalCase for interfaces and classes
- Strong typing with explicit interfaces
- Prefer async/await for async operations
- Import style: `import module from 'module'` for ES modules

### Go
- Standard Go formatting (gofmt)
- CamelCase for exported functions/types
- Comprehensive error handling with explicit returns
- Use goroutines and channels for concurrency
- Document exported functions with comments

## Project Structure
- `/src` - TypeScript source
- `/cycletls` - Go package for TLS fingerprinting
- `/tests` - Test files (mixed JS/TS)
- `/dist` - Output directory for builds

## Communication Pattern
TypeScript code spawns Go process for handling TLS fingerprinting
Critical to properly initialize with `initCycleTLS()` and terminate with `cycleTLS.exit()`