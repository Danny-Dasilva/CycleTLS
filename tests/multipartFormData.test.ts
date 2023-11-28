import initCycleTLS, { CycleTLSClient } from "../dist/index.js";
import FormData from "form-data";
import fs from "fs";
jest.setTimeout(30000);

describe("CycleTLS Multipart Form Data Test", () => {
  let cycleTLS: CycleTLSClient;

  beforeAll(async () => {
    cycleTLS = await initCycleTLS({ port: 9200 });
  });

  afterAll(() => {
    cycleTLS.exit();
  });

  test("Should Handle Multipart Form Data Correctly", async () => {
    const formData = new FormData();
    formData.append("key1", "value1");
    formData.append("key2", "value2");

    const response = await cycleTLS(
      "http://httpbin.org/post",
      {
        body: formData,
      },
      "post"
    );

    expect(response.status).toBe(200); // Check if the status code is 200

    const responseBody =
      typeof response.body === "string"
        ? JSON.parse(response.body)
        : response.body;

    // Validate the 'form' part of the response
    expect(responseBody.form).toEqual({
      key1: "value1",
      key2: "value2",
    });
  });

  test("Should Handle Multipart Form Data with File Upload Correctly", async () => {
    const formData = new FormData();
    const fileStream = fs.createReadStream("./main.go");
    formData.append("file", fileStream);

    const response = await cycleTLS(
      "http://httpbin.org/post",
      {
        body: formData,
        headers: formData.getHeaders(),
      },
      "post"
    );

    expect(response.status).toBe(200);

    const responseBody =
      typeof response.body === "string"
        ? JSON.parse(response.body)
        : response.body;

    expect(responseBody.files).toBeDefined();
    expect(responseBody.files.file).toContain(
      "./cycletls"
    );
  });
});
