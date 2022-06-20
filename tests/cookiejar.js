const initCycleTLS = require("../dist/index.js");

const tough = require("tough-cookie");
const Cookie = tough.Cookie;

(async () => {
  // Initiate cycleTLS and CookieJar
  const cycleTLS = await initCycleTLS();
  const cookieJar = new tough.CookieJar();

  // Send an inital response to demonstrate no cookies being set, and verify it
  const firstResponse = await cycleTLS.get("https://httpbin.org/cookies", {});
  if (firstResponse.body.cookies.length > 0) {
    console.log("Cookies were set.");
    return;
  }

  // Now set a single cookie and make sure HTTPBin responded with a redirect
  const secondResponse = await cycleTLS.get(
    "https://httpbin.org/cookies/set?freeform=test",
    {
      disableRedirect: true,
    }
  );
  if (secondResponse.status !== 302) {
    console.log("HTTPBin could not set cookies successfully.");
    return;
  }
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
  if (thirdResponse.body?.cookies?.freeform === null) {
    console.log("Cookie was not preserved.");
    return;
  }

  // Now send a fourth request setting an additional 3 cookies
  const fourthResponse = await cycleTLS.get(
    "https://httpbin.org/cookies/set?a=1&b=2&c=3",
    {
      disableRedirect: true,
    }
  );
  if (fourthResponse.status !== 302) {
    console.log("HTTPBin could not set cookies successfully.");
    return;
  }
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
  const fifthBody = fifthResponse.body;
  if (
    fifthBody?.cookies?.freeform === null &&
    fifthBody?.cookies?.a &&
    fifthBody?.cookies?.b &&
    fifthBody?.cookies?.c
  ) {
    console.log("Cookie was not preserved.");
    return;
  }

  console.log("Passed all cookie tests successfully.");

  cycleTLS.exit();
})();

async function processCookies(response, url, cookieJar) {
  if (response.headers["Set-Cookie"] instanceof Array) {
    response.headers["Set-Cookie"].map(
      async (cookieString) => await cookieJar.setCookie(cookieString, url)
    );
  } else {
    await cookieJar.setCookie(response.headers["Set-Cookie"], url);
  }
}
