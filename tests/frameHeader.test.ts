import initCycleTLS from "../dist/index.js";

test("Test latest Chrome frame headers", async () => {
  const cycleTLS = await initCycleTLS({ port: 9011 });

  const ja3 =
    "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-17513,29-23-24,0";
  const UA =
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36";

  const response = await cycleTLS('https://tls.peet.ws/api/all', {
    body: '',
    ja3: ja3,
    userAgent: UA,
  });
  const expectedSentFrames0 = {
    frame_type: 'SETTINGS',
    length: 30,
    settings: [
      'HEADER_TABLE_SIZE = 65536',
      'MAX_CONCURRENT_STREAMS = 1000',
      'INITIAL_WINDOW_SIZE = 6291456',
      'MAX_FRAME_SIZE = 16384',
      'MAX_HEADER_LIST_SIZE = 262144',
    ],
  };
  const expectedSentFrames1 = {
    frame_type: 'WINDOW_UPDATE',
    increment: 15663105,
    length: 4,
  };
  
  const result = await response.json();
  expect(result.tls.ja3).toEqual(ja3);
  expect(result.user_agent).toEqual(UA);
  expect(result.http2.sent_frames[0]).toMatchObject(expectedSentFrames0);
  expect(result.http2.sent_frames[1]).toMatchObject(expectedSentFrames1);
  await cycleTLS.exit();
});

test("Test latest Firefox frame headers", async () => {
  const cycleTLS = await initCycleTLS({ port: 9012 });
  const ja3 =
    "771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0";
  const UA =
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:101.0) Gecko/20100101 Firefox/101.0";

  
  const response = await cycleTLS.get("https://tls.peet.ws/api/all", {
    body: "",
    ja3: ja3,
    userAgent: UA,
  });
  const expectedSentFrames0 = {
    frame_type: "SETTINGS",
    length: 18,
    settings: [
      "HEADER_TABLE_SIZE = 65536",
      "INITIAL_WINDOW_SIZE = 131072",
      "MAX_FRAME_SIZE = 16384",
    ],
  };
  const expectedSentFrames1 = {
    frame_type: "WINDOW_UPDATE",
    increment: 12517377,
    length: 4,
  };
  if (typeof (await response.json()) === "object") {
    expect((await response.json())?.tls?.ja3).toEqual(ja3);
    expect((await response.json())?.http2?.sent_frames[0]).toMatchObject(
      expectedSentFrames0
    );
    expect((await response.json())?.http2?.sent_frames[1]).toMatchObject(
      expectedSentFrames1
    );
  } else {
    throw "Object decode error";
  }
  await cycleTLS.exit();
});
