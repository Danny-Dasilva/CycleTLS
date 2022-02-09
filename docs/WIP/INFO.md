https://iana.org/assignments/tls-extensiontype-values/tls-extensiontype-values.xhtml

# get()
<details>

**Description:**
Takes an URL string, makes an HTTP GET and returns a dict with the response.

**Arguments:**
- `url` the remote URL, string type, required, must not be empty string, example `https://dev.to`.
- `user_agent` User Agent, string type, optional, should not be empty string.
- `max_redirects` Maximum Redirects, int type, optional, defaults to `9`, example `5`, example `1`.
- `proxy_url` Proxy URL, string type, optional, if is `""` then NO Proxy is used, defaults to `""`, example `172.15.256.1:666`.
- `proxy_auth` Proxy Auth, string type, optional, if `proxy_url` is `""` then is ignored, defaults to `""`.
- `timeout` Timeout, int type, optional, Milliseconds precision, defaults to `-1`, example `9999`, example `666`.
- `http_headers` HTTP Headers, List of Tuples type, optional, example `[("key", "value")]`, example `[("DNT", "1")]`.

Examples:

```js
```

**Returns:**
Response, `list` type, values of the list are string type,
values of the list can be empty string, the lenght of the list is always 7 items,
the values are like `[body, type, status, version, url, length, headers]`,
you can use `to_json()` to get JSON or `to_dict()` to get a dict or `to_tuples()` to get a tuples.


**See Also:**
[get2str()](https://github.com/juancarlospaco/faster-than-requests#get2str) and [get2str2()](https://github.com/juancarlospaco/faster-than-requests#get2str2)

</details>



# Docker

- Make a quick test drive on Docker!.

```bash
$ ./build-docker.sh
$ ./run-docker.sh
$ ./server4benchmarks &  # Inside Docker.
$ python3 benchmark.py   # Inside Docker.
```


# Dependencies

- **None**


# Platforms

- ✅ Linux
- ✅ Windows
- ✅ Mac