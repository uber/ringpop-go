.PHONY: clean clean-mocks testpop lint mocks out setup test test-integration test-unit test-race

SHELL = /bin/bash

export PATH := $(shell pwd)/scripts/travis/thrift-release/linux-x86_64:$(PATH)
export PATH := $(shell pwd)/scripts/travis/thrift-gen-release/linux-x86_64:$(PATH)

# go commands should use the Godeps/_workspace
GODEPS := $(shell pwd)/Godeps/_workspace
USER_GOPATH := $(GOPATH)
export GOPATH = $(GODEPS):$(USER_GOPATH)

DEV_DEPS = github.com/uber/tchannel-go/thrift/thrift-gen \
		   github.com/vektra/mockery/... \
		   github.com/tools/godep \
		   github.com/golang/lint/golint

# Automatically gather packages
PKGS = $(shell find . -type d -maxdepth 3 \
	! -path '*/.git*' \
	! -path '*/_*' \
	! -path '*/Godeps*' \
	! -path '*/test*' \
	! -path '*/gen-go*' \
)

out:	test

clean:
	rm -f testpop

clean-mocks:
	rm -f test/mocks/*.go forward/mock_*.go
	rm -rf test/thrift/pingpong/

lint:
	@:>lint.log

	@-golint ./... | grep -Ev '(test|gen-go)/' | tee -a lint.log

	@for pkg in $(PKGS); do \
		scripts/lint/run-vet "$$pkg" | tee -a lint.log; \
	done;

	@[ ! -s lint.log ]
	@rm -f lint.log

mocks:
	test/gen-testfiles

setup:
	GOPATH=$(USER_GOPATH) go get -u -v $(DEV_DEPS)
	@if ! which thrift | grep -q /; then \
		echo "thrift not in PATH. (brew install thrift?)" >&2; \
 		exit 1; \
	fi

	ln -sf ../../scripts/pre-commit .git/hooks/pre-commit

test:	test-unit test-integration

test-integration:
	test/run-integration-tests

test-unit:
	go generate ./...
	test/go-test-prettify ./...

test-race:
	go generate ./...
	test/go-test-prettify -race ./...

testpop:	clean
	go build ./scripts/testpop/
