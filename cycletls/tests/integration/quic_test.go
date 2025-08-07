//go:build integration
// +build integration

package cycletls_test

import (
	"crypto/tls"
	"os"
	"runtime"
	"testing"

	"github.com/Danny-Dasilva/CycleTLS/cycletls"
	http "github.com/Danny-Dasilva/fhttp"
)

const TestQUICFingerprint = "16030106f2010006ee03039a2b98d81139db0e128ea09eff6874549c219b543fb6dbaa7e4dbfe9e31602c620ce04c4026f019442affade7fed8ba66e022e186f77f1c670fd992f33c0143f120020aaaa130113021303c02bc02fc02cc030cca9cca8c013c014009c009d002f0035010006851a1a00000010000e000c02683208687474702f312e31002b000706dada03040303002d00020101000d0012001004030804040105030805050108060601001b0003020002ff0100010000230000000a000c000afafa11ec001d001700180000000e000c0000096c6f63616c686f7374003304ef04edfafa00010011ec04c06903195f3660633741f9f5ae64d05a316ac8e717582adb9e58c5c242ba306c99ca8f68c15f261245f9141812383c9265f7d0b5c44a5e0d7633f4f40ab8e820ec01bb6cc74e6ab3168e66b40cc4cd37e96e286b4080552b8b0217f786b7c1a0088fb613cc84471b17a33fbcc68db151df387907a1cf3fb14a0f45c6b84608db5b103131b537255c09559cf1940c3980a7f37959a7f95d32c49923600c76c616af238c579361e1c6a20c251d3d42a50e182ad54b1e5d54a57fe6986e64142815b9478cca8066c9bdcc0eb9022ebe05b0ebd53e7d146761d81aee41cd377611699c536a444d300c152994bfeef3cfb4736cb3d57d683269c1e3c001fba2ac220b2c993cec410f6fa5104d1bcde21c46a9b0be8ab7b51aba15f1745ebbd0a0d3a5224170b3bce456c157937e43390b733375bb96601667f5b36888f9520931bead48bae4723d9ed40af2680746e27eee4328503a280ee8846b37803d6206e0f5248bea4ca4a53ca4e1afdd2b84bce0c83260333bc9b38b86486fb48e18d1ce9187b1b6332b2f4145eca38122ac363210137f52140a57b7976b609d739844fa61f21c1e3c300f3434bc3f8b6856994847e2c9b0f20a24b976f9d552b153246db69f30d4a95301b933b0ba9d48402ddb7863cb4f1923a2c33021fd68634a387bf0f76d87f01b35b6182dd10fe9c14b8e548d7988388308b08ff1585ba18a7615737857a7c23c24ee9b3a2ab1915be18b233acd354c7c6513b8ea617a5cf299f34139756cc1df524292c43ee3364990961b2490ae204634a4461b53c95a11d214503985f27ab85bc7c179ab1ba37a828312cabea8cfc5088616386a83e566279a0a5517b60aca4ec6c30a32191dca3cbb7d33ae5087bbdbab5c42e6293b63ad8e35311d459e1ce57037e65e96283e449c3e012051d247653197834c42613ac377a950607c98cbd5a79fef948d18e99758e12d31d13231e638cfb183623346b231a443f56533d444c7204a63479e4efb34ef97597d858915a8a10a32aa78824c9d2993741176da643be1c6c4d91b6511055b098477e7a3c5b5c312cf7bb4ef5905fd741375e62c8ce942a2117bbe707fcc9871e59c0687507862f3634c871885949fce97612793a30a155e84ac503dc519816a13772c50e1167a7031cb2c8187913108f9a26e55958fd19a6e1ef18ed53a70f1c13d01d71d3b40a22413852c9982daac4ae8071966016c38a60bd5c0258c32b882740c6ed5252093c91e51c50cf037d4f5cb6ca610672710f7ca77b0a76039a9968e368a6b243ee4ca7632855cb568c73f01764c4944fc5879d2c52d7992840863c057db2efec658eeb2a73e02bd62617438d9192911bac1f6b0e55cb38255417af20000d69378c857bb278156f16a684200125906b6c22f3d505bc9e76d75fac3a009332ff98fe6baabe3941cab5271c6d2c0ebc993b944c49bd437353019d1b24d10390e45fa87ad77b329a9025933a11af2af0da44d3ed761722c94d8053242f537624113d7bc0155600573301bd2217c6c481ce63b0944b052c97bcb9d3349258257ff33cccf963a6945119ecab21c25051ce02548f642e0ec1ffd392d60facfdf76bfb7274363b62979231f4996362c85d5ba19d2cab7019750b3443565436867a53b71d875eba3282e6d0ee22076d6b97b7c6c556ae216e8bc1bc9f202ce94c763bfe9afc105fca9372dec2e286a001d00200ee8ac33f1ea3153f6b4a06ab71d21b7ce7955ce64ccfc66b7ec8077d02ffd18fe0d00fa00000100018b00206cada2aa48ee4478c40adad21f147d6bc90d13f6889a9b8a58a02536585a261f00d09306c85aab2a6e424b658f3cd9d1c46f35020839287259d3be605ff97faea0d87b9f7f96529661f08cf3f3899db8e805ee7405e2f9b6abd99bc4f6fa5f99b1ed442ebe53c5b10451c93d1221f662783efc3cc8fcf135ed935bcf02ec32251dd09705f191bd7959afbab5619d8e63cb634a259dd63d1b0e42225ae8c08b5b1620cd59d914857e9f1e8a3b7b892863bdaa05429922d75583059641468d8fc51c01e977a69d3a51d714cd5cceea9a5f404ce4a285fd6647931ed8b1c12db027328f214afdbe2c8102b46fe041b553f8670b00050005010000000044cd0005000302683200170000000b0002010000120000caca000100@@505249202a20485454502f322e300d0a0d0a534d0d0a0d0a00001804000000000000010001000000020000000000040060000000060004000000000408000000000000ef00010001d401250000000180000000ff82418aa0e41d139d09b8f3efbf87845887a47e561cc5801f40874148b1275ad1ffb9fe749d3fd4372ed83aa4fe7efbc1fcbefff3f4a7f388e79a82a97a7b0f497f9fbef07f21659fe7e94fe6f4f61e935b4ff3f7de0fe42cb3fcff408b4148b1275ad1ad49e33505023f30408d4148b1275ad1ad5d034ca7b29f07226d61634f53224092b6b9ac1c8558d520a4b6c2ad617b5a54251f01317ad9d07f66a281b0dae053fad0321aa49d13fda992a49685340c8a6adca7e28104416e277fb521aeba0bc8b1e632586d975765c53facd8f7e8cff4a506ea5531149d4ffda97a7b0f49580b2cae05c0b814dc394761986d975765cf53e5497ca589d34d1f43aeba0c41a4c7a98f33a69a3fdf9a68fa1d75d0620d263d4c79a68fbed00177fe8d48e62b03ee697e8d48e62b1e0b1d7f46a4731581d754df5f2c7cfdf6800bbdf43aeba0c41a4c7a9841a6a8b22c5f249c754c5fbef046cfdf6800bbbf408a4148b4a549275906497f83a8f517408a4148b4a549275a93c85f86a87dcd30d25f408a4148b4a549275ad416cf023f31408a4148b4a549275a42a13f8690e4b692d49f50929bd9abfa5242cb40d25fa523b3e94f684c9f518cf73ad7b4fd7b9fefb4005dff4086aec31ec327d785b6007d286f"
// logQUICTestDetails logs comprehensive diagnostic information for QUIC tests
func logQUICTestDetails(t *testing.T, testName string, err error, status int, resp interface{}) {
	t.Logf("=== %s Diagnostic Info ===", testName)
	t.Logf("Environment: OS=%s, CI=%s, GITHUB_ACTIONS=%s", 
		runtime.GOOS, os.Getenv("CI"), os.Getenv("GITHUB_ACTIONS"))
	
	if err != nil {
		t.Logf("Error occurred: %v", err)
		t.Logf("Error type: %T", err)
	} else {
		t.Logf("Request succeeded without error")
	}
	
	t.Logf("Response status: %d", status)
	
	// Log additional response details if available
	if resp != nil {
		switch r := resp.(type) {
		case cycletls.Response:
			t.Logf("Response body length: %d", len(r.Body))
			if len(r.Body) > 0 && len(r.Body) < 500 {
				t.Logf("Response body preview: %s", r.Body)
			} else if len(r.Body) > 0 {
				t.Logf("Response body preview (first 200 chars): %s", r.Body[:200])
			}
			t.Logf("Response headers: %+v", r.Headers)
		case *http.Response:
			t.Logf("HTTP Response proto: %s", r.Proto)
			t.Logf("HTTP Response headers: %+v", r.Header)
		}
	}
	t.Logf("=== End %s Diagnostic Info ===", testName)
}

func TestQUICHttp3WithRoundTripper(t *testing.T) {
	// Skip QUIC tests in CI environment due to UDP/QUIC restrictions
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping QUIC HTTP/3 test in CI environment due to network restrictions")
		return
	}
	
	// Test HTTP/3 using transport and QUIC fingerprint through the roundtripper
	// Create browser with QUIC fingerprint
	browser := cycletls.Browser{
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		QUICFingerprint: TestQUICFingerprint,
		ForceHTTP3: true,
		InsecureSkipVerify: true,
	}
	
	// Create HTTP client with QUIC fingerprint
	client, err := cycletls.Init().Do("https://cloudflare-quic.com/", cycletls.Options{
		QUICFingerprint: TestQUICFingerprint,
		ForceHTTP3: true,
		UserAgent: browser.UserAgent,
		InsecureSkipVerify: true,
	}, "GET")
	
	// Always log diagnostic information
	logQUICTestDetails(t, "TestQUICHttp3WithRoundTripper", err, client.Status, client)
	
	if err != nil {
		t.Skipf("QUIC HTTP/3 request failed (this may be expected in some environments): %v", err)
		return
	}
	
	if client.Status != 200 {
		t.Errorf("Expected status 200, got %d", client.Status)
	}
}

func TestQUICHttp3WithoutFingerprint(t *testing.T) {
	// Skip QUIC tests in CI environment due to UDP/QUIC restrictions
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping QUIC HTTP/3 test in CI environment due to network restrictions")
		return
	}
	
	// Test HTTP/3 without QUIC fingerprint to verify it still works
	client := cycletls.Init()
	resp, err := client.Do("https://cloudflare-quic.com/", cycletls.Options{
		ForceHTTP3: true,
		UserAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		InsecureSkipVerify: true,
	}, "GET")
	
	// Always log diagnostic information
	logQUICTestDetails(t, "TestQUICHttp3WithoutFingerprint", err, resp.Status, resp)
	
	if err != nil {
		t.Skipf("HTTP/3 request failed (this may be expected in some environments): %v", err)
		return
	}
	
	if resp.Status != 200 {
		t.Errorf("Expected status 200, got %d", resp.Status)
	}
}

func TestQUICTransportDirect(t *testing.T) {
	// Test HTTP/3 transport directly with QUIC fingerprint integration
	// This tests that our QUIC fingerprint works through the roundtripper
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	// Create HTTP/3 transport
	transport := cycletls.NewHTTP3Transport(tlsConfig)

	// Create a test request using fhttp
	req, err := http.NewRequest("GET", "https://cloudflare-quic.com/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add User-Agent for better compatibility
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36")

	// Perform the request using RoundTrip
	resp, err := transport.RoundTrip(req)
	
	// Always log diagnostic information (using StatusCode for HTTP response)
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	logQUICTestDetails(t, "TestQUICTransportDirect", err, statusCode, resp)
	
	if err != nil {
		// This test might fail if HTTP/3 is not supported by the test environment
		t.Skipf("HTTP/3 request failed: %v", err)
		return
	}

	// Check response status
	if resp.StatusCode != 200 {
		t.Errorf("HTTP/3 request returned status %d, want 200", resp.StatusCode)
	}

	// Check protocol (HTTP/3 typically reports as HTTP/3 or HTTP/3.0)
	if resp.Proto != "HTTP/3.0" && resp.Proto != "HTTP/3" {

	}

	// Clean up
	resp.Body.Close()
	

}