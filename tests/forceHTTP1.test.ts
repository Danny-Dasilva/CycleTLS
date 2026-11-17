import CycleTLS from "../dist/index.js";
import { createSuiteInstance, withUpstreamRetry, UPSTREAM_FLAKE_STATUSES } from "./test-utils.js";

describe("CycleTLS HTTP Version Tests", () => {
  let client: CycleTLS;
  let cleanup: () => Promise<void>;
  let ja3 = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0";
  let userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36";

  beforeAll(async () => {
    ({ instance: client, cleanup } = await createSuiteInstance({ port: 9134 }));
  });

  afterAll(async () => {
    await cleanup();
  });

  test("Should use HTTP/2 by default", async () => {
    const response = await withUpstreamRetry(() => client.get('https://tls.peet.ws/api/all', {
      ja3: ja3,
      userAgent: userAgent,
      forceHTTP1: false,
      // tls.peet.ws cert validity is fixture-dependent; we test the outgoing
      // TLS fingerprint, not the test fixture's certificate chain.
      insecureSkipVerify: true,
    }));

    if (UPSTREAM_FLAKE_STATUSES.has(response.status)) {
      console.warn(`tls.peet.ws upstream flake after retries: status ${response.status}, skipping`);
      return;
    }
    expect(response.status).toBe(200);
    const result = await response.json() as { http_version: string };
    expect(result.http_version).toBe('h2');
  });

  test("Should force HTTP/1.1 when specified", async () => {
    const response = await withUpstreamRetry(() => client.get('https://tls.peet.ws/api/all', {
      ja3: ja3,
      userAgent: userAgent,
      forceHTTP1: true,
      insecureSkipVerify: true,
    }));

    if (UPSTREAM_FLAKE_STATUSES.has(response.status)) {
      console.warn(`tls.peet.ws upstream flake after retries: status ${response.status}, skipping`);
      return;
    }
    expect(response.status).toBe(200);
    const result = await response.json() as { http_version: string };
    expect(result.http_version).toBe('HTTP/1.1');
  });
});
