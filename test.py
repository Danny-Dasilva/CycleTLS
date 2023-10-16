import requests
files = {'foo': 'bar'}

print(requests.Request('POST', 'http://httpbin.org/post', files=files).prepare().body.decode('utf8'))
print(requests.Request('POST', 'http://httpbin.org/post', files=files).prepare().body.decode('utf8'))