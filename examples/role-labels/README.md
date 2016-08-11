This application shows a simple use-case of labels where the labels of a node will describe the role the member is fulfilling in the cluster. All nodes are able to list the nodes that fulfill a specific role.

Note: this file can be [cram][3]-executed using `make test-examples`. That's why some of the example outputs below are a bit unusual.

# Running the example

All commands are relative to this directory:

    $ cd ${TESTDIR}  # examples/role-labels

(optional, the files are already included) Generate the thrift code:

    $ go generate

Build the example binary:

    $ go build

Start a custer of 5 nodes using [tick-cluster][1]:

    $ tick-cluster.js --interface=127.0.0.1 -n 5 role-labels &> tick-cluster.log &
    $ sleep 5

Show all nodes that fulfill the `roleA` role using [tcurl][2]:

    $ tcurl role -P hosts.json --thrift ./role.thrift RoleService::GetMembers '{"role":"roleA"}'
    {"ok":true,"head":{},"body":["127.0.0.1:300?","127.0.0.1:300?","127.0.0.1:300?","127.0.0.1:300?","127.0.0.1:300?"],"headers":{"as":"thrift"},"trace":"*"} (glob)

This will give you the list of all instances as they all start with their role set to `roleA`. The role of an instance can be changed by running:

    $ tcurl role -P hosts.json --thrift ./role.thrift RoleService::SetRole '{"role":"roleB"}'
    {"ok":true,"head":{},"headers":{"as":"thrift"},"trace":"*"} (glob)
    $ sleep 2 # give the cluster of 5 some time to converge on the new state of the membership

To validate that this worked we can now lookup both the members for `roleA` and `roleB` and see that the membership lists contain respectively 4 and 1 member:

    $ tcurl role -P hosts.json --thrift ./role.thrift RoleService::GetMembers '{"role":"roleA"}'
    {"ok":true,"head":{},"body":["127.0.0.1:300?","127.0.0.1:300?","127.0.0.1:300?","127.0.0.1:300?"],"headers":{"as":"thrift"},"trace":"*"} (glob)

    $ tcurl role -P hosts.json --thrift ./role.thrift RoleService::GetMembers '{"role":"roleB"}'
    {"ok":true,"head":{},"body":["127.0.0.1:300?"],"headers":{"as":"thrift"},"trace":"*"} (glob)


In the end you should kill tick cluster via:

    $ kill %1

[1]:https://github.com/uber/ringpop-common/
[2]:https://github.com/uber/tcurl
[3]:https://pypi.python.org/pypi/cram
