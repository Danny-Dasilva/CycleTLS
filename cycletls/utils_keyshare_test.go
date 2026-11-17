//go:build !integration

package cycletls

import (
	"testing"

	utls "github.com/refraction-networking/utls"
)

// keyShareGroups extracts the ordered key_share groups from a spec built by
// StringToSpec, or fails if no KeyShareExtension is present.
func keyShareGroups(t *testing.T, ja3, ua string) []utls.CurveID {
	t.Helper()
	spec, err := StringToSpec(ja3, ua, false)
	if err != nil {
		t.Fatalf("StringToSpec(%q, %q): %v", ja3, ua, err)
	}
	for _, ext := range spec.Extensions {
		if ks, ok := ext.(*utls.KeyShareExtension); ok {
			groups := make([]utls.CurveID, len(ks.KeyShares))
			for i, s := range ks.KeyShares {
				groups[i] = s.Group
			}
			return groups
		}
	}
	t.Fatalf("no KeyShareExtension in spec for ja3 %q", ja3)
	return nil
}

func indexOf(groups []utls.CurveID, want utls.CurveID) int {
	for i, g := range groups {
		if g == want {
			return i
		}
	}
	return -1
}

// A JA3 whose supported_groups includes X25519MLKEM768 (4588) must yield a
// key_share carrying an MLKEM768 share positioned before X25519 and after any
// leading GREASE placeholder — matching a real Chrome/Edge post-quantum hello.
func TestKeyShareIncludesMLKEMWhenAdvertised(t *testing.T) {
	pqJA3 := "771,4865-4866,0-23-35-13-10-11-51-43,4588-29-23,0"

	// Chrome: GREASE placeholder(s) precede the real shares.
	chrome := keyShareGroups(t, pqJA3, "chrome")
	mlkem := indexOf(chrome, utls.X25519MLKEM768)
	x25519 := indexOf(chrome, utls.X25519)
	grease := indexOf(chrome, utls.CurveID(utls.GREASE_PLACEHOLDER))
	if mlkem < 0 {
		t.Fatalf("chrome PQ key_share missing X25519MLKEM768: %v", chrome)
	}
	if x25519 < 0 || mlkem >= x25519 {
		t.Fatalf("chrome PQ key_share: MLKEM (%d) must come before X25519 (%d): %v", mlkem, x25519, chrome)
	}
	if grease < 0 || grease >= mlkem {
		t.Fatalf("chrome PQ key_share: GREASE (%d) must precede MLKEM (%d): %v", grease, mlkem, chrome)
	}

	// Non-Chrome: MLKEM still inserted before X25519.
	ff := keyShareGroups(t, pqJA3, "firefox")
	if m, x := indexOf(ff, utls.X25519MLKEM768), indexOf(ff, utls.X25519); m < 0 || x < 0 || m >= x {
		t.Fatalf("non-chrome PQ key_share: MLKEM must precede X25519: %v", ff)
	}
}

// Regression: a JA3 with no post-quantum group must produce the exact legacy
// key_share set — no MLKEM share added — so non-PQ fingerprints are unchanged.
func TestKeyShareUnchangedWithoutPQGroup(t *testing.T) {
	nonPQ := "771,4865-4866,0-23-35-13-10-11-51-43,29-23,0"

	chrome := keyShareGroups(t, nonPQ, "chrome")
	if indexOf(chrome, utls.X25519MLKEM768) != -1 {
		t.Fatalf("chrome non-PQ key_share unexpectedly contains MLKEM: %v", chrome)
	}
	wantChrome := []utls.CurveID{
		utls.CurveID(utls.GREASE_PLACEHOLDER),
		utls.CurveID(utls.GREASE_PLACEHOLDER),
		utls.X25519,
	}
	assertGroups(t, "chrome non-PQ", chrome, wantChrome)

	ff := keyShareGroups(t, nonPQ, "firefox")
	if indexOf(ff, utls.X25519MLKEM768) != -1 {
		t.Fatalf("non-chrome non-PQ key_share unexpectedly contains MLKEM: %v", ff)
	}
	wantFF := []utls.CurveID{
		utls.CurveID(utls.GREASE_PLACEHOLDER),
		utls.X25519,
		utls.CurveP256,
	}
	assertGroups(t, "non-chrome non-PQ", ff, wantFF)
}

func assertGroups(t *testing.T, label string, got, want []utls.CurveID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s key_share: got %v, want %v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s key_share: got %v, want %v", label, got, want)
		}
	}
}
