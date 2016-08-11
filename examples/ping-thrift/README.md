A simple ping-pong service implementation that that integrates ringpop to forward requests between nodes.

Note: this file can be [cram][3]-executed using `make test-examples`. That's why some of the example outputs below are a bit unusual.

# Running the example

All commands are relative to this directory:

    $ cd ${TESTDIR}  # examples/ping-thrift

(optional, the files are already included) Generate the thrift code:

    $ thrift-gen --generateThrift --outputDir gen-go --inputFile ping.thrift

Build the example binary:

    $ go build


Start a custer of 5 nodes using [tick-cluster][1]:

    $ tick-cluster.js --interface=127.0.0.1 -n 5 ping-thrift &> tick-cluster.log &
    $ sleep 5

Lookup the node `my_key` key belongs to using [tcurl][2]:

    $ tcurl ringpop -P hosts.json /admin/lookup '{"key": "my_key"}'
    {"ok":true,"head":null,"body":{"dest":"127.0.0.1:300?"},"headers":{"as":"json"},"trace":"*"} (glob)

Call the `PingPongService::Ping` endpoint (multiple times) and see the request being forwarded. Each request is sent to a random node in the cluster because of the `-P hosts.json` argument--but is always handled by the node owning the key. This can be seen in the `from` field of the response:

    $ tcurl pingchannel -P hosts.json --thrift ./ping.thrift PingPongService::Ping '{"request": {"key": "my_key"}}'
    {"ok":true,"head":{},"body":{"message":"Hello, world!","from_":"127.0.0.1:300?","pheader":""},"headers":{"as":"thrift"},"trace":"*"} (glob)

Optionally, set the `p` header. This value will be forwarded together with the request body to the node owning the key. Its value is returned in the response body in the `pheader` field:

    $ tcurl pingchannel -P hosts.json --thrift ./ping.thrift PingPongService::Ping '{"request": {"key": "my_key"}}' --headers '{"p": "my_header"}'
    {"ok":true,"head":{},"body":{"message":"Hello, world!","from_":"127.0.0.1:300?","pheader":"my_header"},"headers":{"as":"thrift"},"trace":"*"} (glob)

    $ kill %1

[1]:https://github.com/uber/ringpop-common/
[2]:https://github.com/uber/tcurl
[3]:https://pypi.python.org/pypi/cram
