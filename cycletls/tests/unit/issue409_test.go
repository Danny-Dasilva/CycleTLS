package unit

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	utls "github.com/refraction-networking/utls"
)

// The JA4R string from issue 409 (Chrome 138, includes ECH 0xfe0d).
const issue409JA4R = "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601"

func TestIssue409_JA4RHandshake(t *testing.T) {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	ts.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
	}
	ts.StartTLS()
	defer ts.Close()

	client := cycletls.Init()
	defer client.Close()

	resp, err := client.Do(ts.URL, cycletls.Options{
		Ja4r:               issue409JA4R,
		UserAgent:          "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36",
		InsecureSkipVerify: true,
	}, "GET")

	if err != nil {
		t.Fatalf("client.Do failed: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("expected status 200, got %d, body: %s", resp.Status, resp.Body)
	}
}

func TestIssue409_CreateExtensionECH(t *testing.T) {
	// 0xfe0d is a real extension ID that JA4 counts, so it must be sent with
	// or without disableGrease.
	for _, disableGrease := range []bool{false, true} {
		ext := cycletls.CreateExtensionFromID(0xfe0d, utls.VersionTLS13, nil, disableGrease, "example.com")
		if _, ok := ext.(*utls.GREASEEncryptedClientHelloExtension); !ok {
			t.Fatalf("disableGrease=%v: expected *utls.GREASEEncryptedClientHelloExtension, got %T", disableGrease, ext)
		}
	}
}

// disableGrease exists for exact JA4 matching, so it must not change how many
// extensions a JA4R string produces.
func TestIssue409_DisableGreaseKeepsExtensionCount(t *testing.T) {
	counts := map[bool]int{}
	for _, disableGrease := range []bool{false, true} {
		spec, err := cycletls.JA4RStringToSpec(issue409JA4R, "", false, disableGrease, "example.com")
		if err != nil {
			t.Fatalf("disableGrease=%v: JA4RStringToSpec failed: %v", disableGrease, err)
		}
		counts[disableGrease] = len(spec.Extensions)
	}
	// 15 extensions listed in the JA4R string plus ALPN from the "h2" header.
	if counts[false] != 16 || counts[true] != 16 {
		t.Fatalf("expected 16 extensions with and without disableGrease, got %d and %d", counts[false], counts[true])
	}
}

// Servers such as Cloudflare answer extension 0x001b with a
// CompressedCertificate message. uTLS only accepts that message when the
// extension is its own UtlsCompressCertExtension type.
func TestIssue409_CompressCertificateUsesUTLSType(t *testing.T) {
	ext := cycletls.CreateExtensionFromID(0x001b, utls.VersionTLS13, nil, false, "example.com")
	ccExt, ok := ext.(*utls.UtlsCompressCertExtension)
	if !ok {
		t.Fatalf("expected *utls.UtlsCompressCertExtension, got %T", ext)
	}
	if len(ccExt.Algorithms) != 1 || ccExt.Algorithms[0] != utls.CertCompressionBrotli {
		t.Fatalf("expected brotli only, got %v", ccExt.Algorithms)
	}
}

func TestIssue409_SupportedCurvesNoDuplicate(t *testing.T) {
	ext := cycletls.CreateExtensionFromID(0x000a, utls.VersionTLS13, nil, false, "example.com")
	if ext == nil {
		t.Fatal("expected non-nil SupportedCurves extension")
	}
	curvesExt, ok := ext.(*utls.SupportedCurvesExtension)
	if !ok {
		t.Fatalf("expected *utls.SupportedCurvesExtension, got %T", ext)
	}

	seen := make(map[utls.CurveID]bool)
	for _, c := range curvesExt.Curves {
		if seen[c] {
			t.Fatalf("duplicate curve %v found in SupportedCurvesExtension: %v", c, curvesExt.Curves)
		}
		seen[c] = true
	}

	if len(curvesExt.Curves) == 0 || curvesExt.Curves[0] != utls.X25519MLKEM768 {
		t.Fatalf("expected first curve to be X25519MLKEM768 in TLS 1.3, got %v", curvesExt.Curves)
	}
}

func TestIssue409_KeyShare(t *testing.T) {
	ext := cycletls.CreateExtensionFromID(0x0033, utls.VersionTLS13, nil, false, "example.com")
	if ext == nil {
		t.Fatal("expected non-nil KeyShare extension")
	}
	ksExt, ok := ext.(*utls.KeyShareExtension)
	if !ok {
		t.Fatalf("expected *utls.KeyShareExtension, got %T", ext)
	}
	if len(ksExt.KeyShares) < 2 {
		t.Fatalf("expected at least 2 key shares, got %d", len(ksExt.KeyShares))
	}
	if ksExt.KeyShares[0].Group != utls.X25519MLKEM768 {
		t.Fatalf("expected first key share to be X25519MLKEM768, got %v", ksExt.KeyShares[0].Group)
	}
	if ksExt.KeyShares[1].Group != utls.X25519 {
		t.Fatalf("expected second key share to be X25519, got %v", ksExt.KeyShares[1].Group)
	}
}
