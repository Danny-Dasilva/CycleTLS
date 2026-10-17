package unit

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	utls "github.com/refraction-networking/utls"
)

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

	ja4r := "t13d1516h2_002f,0035,009c,009d,1301,1302,1303,c013,c014,c02b,c02c,c02f,c030,cca8,cca9_0000,0005,000a,000b,000d,0012,0017,001b,0023,002b,002d,0033,44cd,fe0d,ff01_0403,0804,0401,0503,0805,0501,0806,0601"

	resp, err := client.Do(ts.URL, cycletls.Options{
		Ja4r:               ja4r,
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
	// When GREASE is enabled, 0xfe0d should return valid BoringGREASEECH extension
	ext := cycletls.CreateExtensionFromID(0xfe0d, utls.VersionTLS13, nil, false, "example.com")
	if ext == nil {
		t.Fatal("expected non-nil extension for 0xfe0d when disableGrease is false")
	}
	if _, ok := ext.(*utls.GREASEEncryptedClientHelloExtension); !ok {
		t.Fatalf("expected *utls.GREASEEncryptedClientHelloExtension, got %T", ext)
	}

	// When GREASE is disabled, 0xfe0d should return nil
	extDisabled := cycletls.CreateExtensionFromID(0xfe0d, utls.VersionTLS13, nil, true, "example.com")
	if extDisabled != nil {
		t.Fatalf("expected nil extension for 0xfe0d when disableGrease is true, got %T", extDisabled)
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
