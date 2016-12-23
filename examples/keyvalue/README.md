This application shows a complex example where a node makes requests to itself from a sharded endpoint.

Note: this file can be [cram][3]-executed using `make test-examples`. That's why some of the example outputs below are a bit unusual.

# Running the example

All commands are relative to this directory:

    $ cd ${TESTDIR}  # examples/keyvalue

(optional, the files are already included) Generate the thrift code:

    $ go generate

Build the example binary:

    $ go build

Start a custer of 5 nodes using [tick-cluster][1]:

    $ tick-cluster.js --interface=127.0.0.1 -n 5 keyvalue &> tick-cluster.log &
    $ sleep 5

Set some reference keys that are sharded around the cluster using [tcurl][2]:

    $ tcurl keyvalue -P hosts.json --thrift ./keyvalue.thrift KeyValueService::Set '{"key":"127.0.0.1:30010", "value": "foo"}' # key 127.0.0.1:30010 is the first replica point on the node with identity `127.0.0.1:3001`. The identity is comprised of the host:port + replica index eg '127.0.0.1:3001' + '0'
    {"ok":true,"head":{},"headers":{"as":"thrift"},"trace":"*"} (glob)
    $ tcurl keyvalue -P hosts.json --thrift ./keyvalue.thrift KeyValueService::Set '{"key":"127.0.0.1:30020", "value": "bar"}'
    {"ok":true,"head":{},"headers":{"as":"thrift"},"trace":"*"} (glob)
    $ tcurl keyvalue -P hosts.json --thrift ./keyvalue.thrift KeyValueService::Set '{"key":"127.0.0.1:30040", "value": "baz"}'
    {"ok":true,"head":{},"headers":{"as":"thrift"},"trace":"*"} (glob)

Use GetAll on the node that should answer to make sure self requests work if the first call is not forwarded

    $ tcurl keyvalue -p 127.0.0.1:3004 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30010","127.0.0.1:30020"]}'
    {"ok":true,"head":{},"body":["foo","bar"],"headers":{"as":"thrift"},"trace":"*"} (glob)

Use GetAll on the node that should not answer to make sure self requests work after forwarding

    $ tcurl keyvalue -p 127.0.0.1:3000 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30010","127.0.0.1:30020"]}'
    {"ok":true,"head":{},"body":["foo","bar"],"headers":{"as":"thrift"},"trace":"*"} (glob)

Now do the same but also with a key stored on the node executing the fanout

    $ tcurl keyvalue -p 127.0.0.1:3004 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30010","127.0.0.1:30020","127.0.0.1:30040"]}'
    {"ok":true,"head":{},"body":["foo","bar","baz"],"headers":{"as":"thrift"},"trace":"*"} (glob)
    $ tcurl keyvalue -p 127.0.0.1:3000 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30010","127.0.0.1:30020","127.0.0.1:30040"]}'
    {"ok":true,"head":{},"body":["foo","bar","baz"],"headers":{"as":"thrift"},"trace":"*"} (glob)

And to top it off we will now lookup the local key first

    $ tcurl keyvalue -p 127.0.0.1:3004 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30040","127.0.0.1:30010","127.0.0.1:30020"]}'
    {"ok":true,"head":{},"body":["baz","foo","bar"],"headers":{"as":"thrift"},"trace":"*"} (glob)
    $ tcurl keyvalue -p 127.0.0.1:3000 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30040","127.0.0.1:30010","127.0.0.1:30020"]}'
    {"ok":true,"head":{},"body":["baz","foo","bar"],"headers":{"as":"thrift"},"trace":"*"} (glob)

In the end you should kill tick cluster via:

    $ kill %1

[1]:https://github.com/uber/ringpop-common/
[2]:https://github.com/uber/tcurl
[3]:https://pypi.python.org/pypi/cram
