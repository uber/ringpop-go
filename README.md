ringpop-go [![Build Status](https://travis-ci.org/uber/ringpop-go.svg?branch=master)](https://travis-ci.org/uber/ringpop-go) [![Coverage Status](https://coveralls.io/repos/uber/ringpop-go/badge.svg?branch=master&service=github)](https://coveralls.io/github/uber/ringpop-go?branch=master)
==========

**(This project is no longer under active development.)**

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

First make certain that `thrift` (OSX: `brew install thrift`) and `glide` are
in your path (above). Then,

```
make setup
```

to install remaining golang dependencies and install the pre-commit hook.

Finally, run the tests by doing:

```
make test
```

Documentation
--------------

Interested in where to go from here? Read the docs at
[ringpop.readthedocs.org](https://ringpop.readthedocs.org)
