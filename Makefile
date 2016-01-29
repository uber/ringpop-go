.PHONY: clean clean-mocks testpop mocks out setup test test-integration test-unit

SHELL = /bin/bash

export PATH := $(shell pwd)/scripts/travis/thrift-release/linux-x86_64:$(PATH)
export PATH := $(shell pwd)/scripts/travis/thrift-gen-release/linux-x86_64:$(PATH)

# go commands should use the Godeps/_workspace
GODEPS := $(shell pwd)/Godeps/_workspace
USER_GOPATH := $(GOPATH)
export GOPATH = $(GODEPS):$(USER_GOPATH)

DEV_DEPS = github.com/uber/tchannel-go/thrift/thrift-gen github.com/vektra/mockery/...

out:	test

clean:
	rm -f testpop ping-ring

clean-mocks:
	rm -f test/mocks/*.go forward/mock_*.go
	rm -rf test/thrift/pingpong/

mocks:
	test/gen-testfiles

setup:
	GOPATH=$(USER_GOPATH) go get -v $(DEV_DEPS)
	@if ! which thrift | grep -q /; then \
		echo "thrift not in PATH. (brew install thrift?)" >&2; \
 		exit 1; \
	fi

test:	test-unit test-integration

test-integration:
	test/run-integration-tests

test-unit:
	go generate ./...
	go test -v ./... |test/go-test-prettify

testpop:	clean
	go build scripts/testpop/testpop.go

ping-ring:	clean
	go build -o ping-ring examples/ping-ring/main.go

ping-ring-debug:	clean
	godebug build -instrument github.com/uber/ringpop-go,github.com/uber/ringpop-go/hashring -o ping-ring examples/ping-ring/main.go
