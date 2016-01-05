ringpop-go [![Build Status](https://travis-ci.org/uber/ringpop-go.svg?branch=master)](https://travis-ci.org/uber/ringpop-go)
==========

Ringpop is a library that brings cooperation and coordination to distributed
applications. It maintains a consistent hash ring on top of a membership
protocol and provides request forwarding as a routing convenience. It can be
used to shard your application in a way that's scalable and fault tolerant.

Getting started
---------------

To install ringpop-go:

```
go get github.com/uber/ringpop-go
```

Developing
----------

Run the tests by doing:

```
make
```

To run the test you need both `thrift` and `thrift-gen` on your path. On OSX you
can install them with the following commands:

```bash
brew install thrift
go get github.com/uber/tchannel-go/thrift/thrift-gen
```

Documentation
--------------

Interested in where to go from here? Read the docs at
[ringpop.readthedocs.org](https://ringpop.readthedocs.org)
