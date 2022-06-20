const initCycleTLS = require("../dist/index.js");

const tough = require("tough-cookie");
const Cookie = tough.Cookie;

(async () => {
  // Initiate cycleTLS and CookieJar
  const cycleTLS = await initCycleTLS();
  const cookieJar = new tough.CookieJar();

  // Capture a set cookie
  const firstResponse = await cycleTLS.get(
    "https://httpbin.org/cookies/set?freeform=test",
    {
      disableRedirect: true,
    }
  );
  
  // Now use the processCookies function to add the cookies from the response headers to the cookie jar
  await processCookies(
    firstResponse,
    "https://httpbin.org/cookies/set?freeform=test",
    cookieJar
  );
  // Now send a second to verify we have our cookies
  const secondResponse = await cycleTLS.get("https://httpbin.org/cookies", {
    headers: {
      cookie: await cookieJar.getCookieString("https://httpbin.org/cookies"),
    },
  });
  
  //verify cookies were set
  console.log(secondResponse.body)
  /* Expected
  {
    "cookies": {
      "freeform": "test"
    }
  }
  */
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
