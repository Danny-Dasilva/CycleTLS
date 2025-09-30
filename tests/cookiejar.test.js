const initCycleTLS = require("../dist/index.js");
const { withCycleTLS } = require("./test-utils.js");
jest.setTimeout(30000);

const tough = require("tough-cookie");
const Cookie = tough.Cookie;

test("Should properly set and configure cookies", async () => {
  await withCycleTLS({ port: 9991 }, async (cycleTLS) => {
    const cookieJar = new tough.CookieJar();

    // Send an inital response to demonstrate no cookies being set, and verify it
    const firstResponse = await cycleTLS.get("https://httpbin.org/cookies");
    //check if cookies were returned
    expect(firstResponse.data.cookies?.length).toBe(undefined);

    // Now set a single cookie and make sure HTTPBin responded with a redirect
    const secondResponse = await cycleTLS.get(
      "https://httpbin.org/cookies/set?freeform=test",
      {
        disableRedirect: true,
      }
    );
    expect(secondResponse.status).toEqual(302);

    // Now use the processCookies function to add the cookies from the response headers to the cookie jar
    await processCookies(
      secondResponse,
      "https://httpbin.org/cookies/set?freeform=test",
      cookieJar
    );

    // Now send a third request back to /cookies to verify we still have our cookies
    const thirdResponse = await cycleTLS.get("https://httpbin.org/cookies", {
      headers: {
        cookie: await cookieJar.getCookieString("https://httpbin.org/cookies"),
      },
    });
    thirdResponseJson = await thirdResponse.json();
    expect(thirdResponseJson?.cookies?.freeform).not.toEqual(null);

    // Now send a fourth request setting an additional 3 cookies
    const fourthResponse = await cycleTLS.get(
      "https://httpbin.org/cookies/set?a=1&b=2&c=3",
      {
        disableRedirect: true,
      }
    );
    expect(secondResponse.status).toEqual(302);

    // Process them again
    await processCookies(
      fourthResponse,
      "https://httpbin.org/cookies/set?freeform=test",
      cookieJar
    );

    // Now verify all the cookies exist
    const fifthResponse = await cycleTLS.get("https://httpbin.org/cookies", {
      headers: {
        cookie: await cookieJar.getCookieString("https://httpbin.org/cookies"),
      },
    });
    const expected_cookies = { a: "1", b: "2", c: "3", freeform: "test" };
    const fifthBody = await fifthResponse.json()

    expect(fifthBody?.cookies).toEqual(expected_cookies);
  });
});

async function processCookies(response, url, cookieJar) {
  // Check for both "Set-Cookie" and "set-cookie" (lowercase)
  const setCookieHeader = response.headers["Set-Cookie"] || response.headers["set-cookie"];
  
  if (!setCookieHeader) {
    // No cookies to process
    return;
  }
  
  if (setCookieHeader instanceof Array) {
    // Process array of cookies
    await Promise.all(
      setCookieHeader.map(cookieString => cookieJar.setCookie(cookieString, url))
    );
  } else {
    // Process single cookie
    await cookieJar.setCookie(setCookieHeader, url);
  }
}
