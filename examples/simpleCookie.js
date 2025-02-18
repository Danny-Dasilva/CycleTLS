const initCycleTLS = require("../dist/index.js");

(async () => {
  // Initiate cycleTLS
  const cycleTLS = await initCycleTLS();
  const response = await cycleTLS("https://httpbin.org/cookies", {
    cookies: {
      cookie1: "value1",
      cookie2: "value2",
    },
  });
  console.log(await response.text());
  /* Expected
  {
    "cookies": {
      "cookie1": "value1",
      "cookie2": "value2"
    }
  }
  */
  await cycleTLS.exit();
})();
