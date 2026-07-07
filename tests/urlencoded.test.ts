import CycleTLS from "../dist/index.js";
import { withCycleTLS, streamToJson, withUpstreamRetry, UPSTREAM_FLAKE_STATUSES } from "./test-utils";

// 30s per-test deadline so a hung httpbin doesn't drag the whole CI job past
// the 6h runner kill (root cause of the soft-fail Node.js Integration job
// timing out for hours). Default Jest is 5s — bumped here to give the retry
// helper its full ~3.5s budget plus normal request latency.
jest.setTimeout(30000);

test("Should Handle URL Encoded Form Data Correctly", async () => {
  await withCycleTLS({ port: 9200, timeout: 10000 }, async (cycleTLS) => {
    const urlEncodedData = new URLSearchParams();
    urlEncodedData.append("key1", "value1");
    urlEncodedData.append("key2", "value2");

    const response = await withUpstreamRetry(() =>
      cycleTLS.post(
        "http://httpbin.org/post",
        urlEncodedData.toString(),
        {
          headers: {
            "Content-Type": "application/x-www-form-urlencoded",
          },
        }
      )
    );

    if (UPSTREAM_FLAKE_STATUSES.has(response.status)) {
      console.log(`Skipped: httpbin upstream flake (status ${response.status} after retries)`);
      return;
    }

    const responseBody = await streamToJson<{ form: Record<string, string> }>(response.body);

    // Validate the 'form' part of the response
    expect(responseBody.form).toEqual({
      key1: "value1",
      key2: "value2",
    });
  });
});
