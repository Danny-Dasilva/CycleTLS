import CycleTLS from "../dist/index.js";
import { createSafeCycleTLS, cleanupCycleTLS, withUpstreamRetry, UPSTREAM_FLAKE_STATUSES } from "./test-utils.js";

jest.setTimeout(30000);

test("Test Multiple Imports, should return 200", async () => {
  const url = "https://httpbin.org/get";
  const options = {
    ja3: "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-51-57-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
    userAgent:
      "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:87.0) Gecko/20100101 Firefox/87.0",
  };

  // Test multiple imports by using the same port - second should connect as client
  const client1 = await createSafeCycleTLS({ port: 9161, timeout: 10000 });
  const client2 = await createSafeCycleTLS({ port: 9161, timeout: 10000 });

  try {
    // Both instances should work — wrap in upstream-flake retry; if httpbin
    // is rate-limiting / 502-ing we skip rather than fail the soft-fail tier.
    const resp1 = await withUpstreamRetry(() => client1.get(url, options));
    if (UPSTREAM_FLAKE_STATUSES.has(resp1.status)) {
      console.log(`Skipped: httpbin upstream flake on client1 (status ${resp1.status})`);
      return;
    }
    expect(resp1.status).toBe(200);

    const resp2 = await withUpstreamRetry(() => client2.get(url, options));
    if (UPSTREAM_FLAKE_STATUSES.has(resp2.status)) {
      console.log(`Skipped: httpbin upstream flake on client2 (status ${resp2.status})`);
      return;
    }
    expect(resp2.status).toBe(200);
  } finally {
    await cleanupCycleTLS(client2);
    await cleanupCycleTLS(client1);
  }
});
