import initCycleTLS, { CycleTLSResponse } from "../dist/index.js";
import { withCycleTLS } from "./test-utils.js";

test("Should Handle URL Encoded Form Data Correctly", async () => {
  await withCycleTLS({ port: 9200 }, async (cycleTLS) => {
    const urlEncodedData = new URLSearchParams();
    urlEncodedData.append("key1", "value1");
    urlEncodedData.append("key2", "value2");

    const response = await cycleTLS(
      "http://httpbin.org/post",
      {
        body: urlEncodedData.toString(),
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
        },
      },
      "post"
    );
    const responseBody = await response.json();

    // Validate the 'form' part of the response
    expect(responseBody.form).toEqual({
      key1: "value1",
      key2: "value2",
    });
  });
});
