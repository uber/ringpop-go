This application shows a complex example where a node makes requests to its self from a sharded endpoint.

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

    $ tcurl keyvalue -P hosts.json --thrift ./keyvalue.thrift KeyValueService::Set '{"key":"127.0.0.1:30010", "value": "foo"}'
    {"ok":true,"head":{},"headers":{"as":"thrift"},"trace":"*"} (glob)
    $ tcurl keyvalue -P hosts.json --thrift ./keyvalue.thrift KeyValueService::Set '{"key":"127.0.0.1:30020", "value": "bar"}'
    {"ok":true,"head":{},"headers":{"as":"thrift"},"trace":"*"} (glob)
    $ tcurl keyvalue -P hosts.json --thrift ./keyvalue.thrift KeyValueService::Set '{"key":"127.0.0.1:30040", "value": "baz"}'
    {"ok":true,"head":{},"headers":{"as":"thrift"},"trace":"*"} (glob)

Use GetAll on the node that should answer to make sure self requests work if the first call is not forwarded

	$ tcurl keyvalue -p 127.0.0.1:3004 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30010","127.0.0.1:30020"]}'
	{"ok":true,"head":{},"body":{"127.0.0.1:30010":"foo","127.0.0.1:30020":"bar"},"headers":{"as":"thrift"},"trace":"*"}

Use GetAll on the node that should not answer to make sure self requests work after forwarding

	$ tcurl keyvalue -p 127.0.0.1:3000 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30010","127.0.0.1:30020"]}'
	{"ok":true,"head":{},"body":{"127.0.0.1:30010":"foo","127.0.0.1:30020":"bar"},"headers":{"as":"thrift"},"trace":"*"}

Now do the same but also with a key stored on the node executing the fanout
	$ tcurl keyvalue -p 127.0.0.1:3004 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30010","127.0.0.1:30020","127.0.0.1:30040"]}'
	{"ok":true,"head":{},"body":{"127.0.0.1:30010":"foo","127.0.0.1:30020":"bar","127.0.0.1:30040":"baz"},"headers":{"as":"thrift"},"trace":"*"}
	$ tcurl keyvalue -p 127.0.0.1:3000 --thrift ./keyvalue.thrift KeyValueService::GetAll '{"keys":["127.0.0.1:30010","127.0.0.1:30020","127.0.0.1:30040"]}'
	{"ok":true,"head":{},"body":{"127.0.0.1:30010":"foo","127.0.0.1:30020":"bar","127.0.0.1:30040":"baz"},"headers":{"as":"thrift"},"trace":"*"}

In the end you should kill tick cluster via:

    $ kill %1

[1]:https://github.com/uber/ringpop-common/
[2]:https://github.com/uber/tcurl
[3]:https://pypi.python.org/pypi/cram
