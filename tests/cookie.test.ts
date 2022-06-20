import initCycleTLS from "../dist/index.js";

test("Simple Cookie test", async () => {
  const cycleTLS = await initCycleTLS({port: 9093});

  const cookies = {
    cookie1: "value1",
    cookie2: "value2",
  };

  const response = cycleTLS("https://httpbin.org/cookies", {
    cookies: cookies,
  });

  await response.then((out) => {
    expect(typeof out.body === "object" ? out.body?.cookies : out.body).toEqual(
      cookies
    ); //Process request
  });
  cycleTLS.exit();
});

test("Complex Cookie test", async () => {
  const cycleTLS = await initCycleTLS({port: 9092});
  const cookies = {
    cookie1: "value1",
    cookie2: "value2",
  };
  const complexCookies = [
    {
      name: "cookie1",
      value: "value1",
    },
    {
      name: "cookie2",
      value: "value2",
    },
  ];

  const response = cycleTLS("https://httpbin.org/cookies", {
    cookies: complexCookies,
  });

  await response.then((out) => {
    expect(typeof out.body === "object" ? out.body?.cookies : out.body).toEqual(
      cookies
    ); //Process request
  });
  cycleTLS.exit();
});
