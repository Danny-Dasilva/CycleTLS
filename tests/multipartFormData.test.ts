import CycleTLS from "../dist/index.js";
import FormData from "form-data";
import fs from "fs";
import { createSuiteInstance, withUpstreamRetry, UPSTREAM_FLAKE_STATUSES } from "./test-utils.js";

// 30s per-test deadline — see urlencoded.test.ts for context.
jest.setTimeout(30000);

describe("CycleTLS Multipart Form Data Test", () => {
  let client: CycleTLS;
  let cleanup: () => Promise<void>;

  beforeAll(async () => {
    ({ instance: client, cleanup } = await createSuiteInstance({ port: 9160, timeout: 10000 }));
  });

  afterAll(async () => {
    await cleanup();
  });

  test("Should Handle Multipart Form Data Correctly", async () => {
    const formData = new FormData();
    formData.append("key1", "value1");
    formData.append("key2", "value2");

    const response = await withUpstreamRetry(() =>
      client.post(
        "http://httpbin.org/post",
        formData.getBuffer().toString(),
        {
          headers: formData.getHeaders(),
        }
      )
    );

    if (UPSTREAM_FLAKE_STATUSES.has(response.status)) {
      console.log(`Skipped: httpbin upstream flake (status ${response.status} after retries)`);
      return;
    }
    expect(response.status).toBe(200);

    const responseBody = await response.json() as { form: { key1: string; key2: string } };

    // Validate the 'form' part of the response
    expect(responseBody.form).toEqual({
      key1: "value1",
      key2: "value2",
    });
  });

  test("Should Handle Multipart Form Data with File Upload Correctly", async () => {
    const formData = new FormData();
    const fileContent = fs.readFileSync("./main.go");
    formData.append("file", fileContent, { filename: "main.go" });

    const response = await withUpstreamRetry(() =>
      client.post(
        "http://httpbin.org/post",
        formData.getBuffer().toString(),
        {
          headers: formData.getHeaders(),
        }
      )
    );

    if (UPSTREAM_FLAKE_STATUSES.has(response.status)) {
      console.log(`Skipped: httpbin upstream flake (status ${response.status} after retries)`);
      return;
    }
    expect(response.status).toBe(200);

    const responseBody = await response.json();

    const body = responseBody as { files: { file: string } };
    expect(body.files).toBeDefined();
    expect(body.files.file).toContain(
      "imports locally per go.mod"
    );
  });
});
