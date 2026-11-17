module github.com/Danny-Dasilva/CycleTLS/cycletls

go 1.26.0

toolchain go1.26.11

require (
	github.com/Danny-Dasilva/fhttp v0.0.0-20260106165651-41258808b131
	github.com/andybalholm/brotli v1.2.7
	github.com/gorilla/websocket v1.5.3
	github.com/klauspost/compress v1.20.2
	github.com/quic-go/quic-go v0.59.1
	github.com/refraction-networking/uquic v0.0.6
	github.com/refraction-networking/utls v1.8.2
	golang.org/x/net v0.60.0
	golang.org/x/sync v0.23.0
	h12.io/socks v1.0.3
)

require (
	github.com/gaukas/clienthellod v0.4.2 // indirect
	github.com/gaukas/godicttls v0.0.4 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/pprof v0.0.0-20250630185457-6e76a2b096b5 // indirect
	github.com/onsi/ginkgo/v2 v2.23.4 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/mock v0.5.2 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/exp v0.0.0-20250718183923-645b1fa84792 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
)

// Self-replace ensures local development uses the local module rather than
// a published version, required because src/go.mod references this module path.
replace github.com/Danny-Dasilva/CycleTLS/cycletls => ./
