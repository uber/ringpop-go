A simple ping-pong service implementation that that integrates ringpop to forward requests between nodes.

# Running the example

All commands are relative to this directory:
```bash
cd examples/ping-thrift
```

(optional, the files are already included) Generate the thrift code:
```bash
thrift-gen --generateThrift --outputDir gen-go --inputFile ping.thrift
```

Build the example binary:
```bash
go build
```

Start a custer of 5 nodes using [tick-cluster][1]:
```bash
tick-cluster.js -n 5 ping-thrift
```

Lookup the node `my_key` key belongs to using [tcurl][2]:
```bash
$ tcurl ringpop -P hosts.json /admin/lookup '{"key": "my_key"}'
{"ok":true,"head":null,"body":{"dest":"127.0.0.1:3002"},"headers":{"as":"json"},"trace":"7a612e506428cec2"}
```

Call the `PingPongService::Ping` endpoint (multiple times) and see the request being forwarded. Each request is sent to a random node in the cluster because of the `-P hosts.json` argument--but is always handled by the node owning the key. This can be seen in the `from` field of the response:
```bash
$ tcurl pingchannel -P hosts.json --thrift ./ping.thrift PingPongService::Ping '{"request": {"key": "my_key"}}'
{"ok":true,"head":{},"body":{"message":"Hello, world!","from_":"127.0.0.1:3002","pheader":""},"headers":{"as":"thrift"},"trace":"650cbf0656e215e2"}
```

Optionally, set the `p` header. This value will be forwarded together with the request body to the node owning the key. Its value is returned in the response body in the `pheader` field:
```bash
$ tcurl pingchannel -P hosts.json /ping '{"key": "my_key"}' --headers '{"p": "my_header"}'
{"ok":true,"head":{},"body":{"message":"Hello, world!","from_":"127.0.0.1:3002","pheader":"my_header"},"headers":{"as":"thrift"},"trace":"b5526f9625a88347"}
```

[1]:https://github.com/uber/ringpop-common/
[2]:https://github.com/uber/tcurl
