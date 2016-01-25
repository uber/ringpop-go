.PHONY: clean clean-mocks testpop mocks out test test-integration test-unit

SHELL = /bin/bash

export PATH := $(shell pwd)/scripts/travis/thrift-release/linux-x86_64:$(PATH)
export PATH := $(shell pwd)/scripts/travis/thrift-gen-release/linux-x86_64:$(PATH)

# go commands should use the Godeps/_workspace
GODEPS := $(shell pwd)/Godeps/_workspace
OLDGOPATH := $(GOPATH)
export GOPATH = $(GODEPS):$(OLDGOPATH)

out:	test

clean:
	rm -f testpop

clean-mocks:
	rm -f test/mocks/*.go forward/mock_*.go
	rm -rf test/thrift/pingpong/

mocks:
	test/gen-testfiles

test:	test-unit test-integration

test-integration:
	test/run-integration-tests

test-unit:
	go generate ./...
	go test -v ./... |test/go-test-prettify

testpop:	clean
	go build scripts/testpop/testpop.go
