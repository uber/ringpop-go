.PHONY: clean clean-mocks testpop lint mocks out setup test test-integration test-unit

SHELL = /bin/bash

export PATH := $(shell pwd)/scripts/travis/thrift-release/linux-x86_64:$(PATH)
export PATH := $(shell pwd)/scripts/travis/thrift-gen-release/linux-x86_64:$(PATH)

# go commands should use the Godeps/_workspace
GODEPS := $(shell pwd)/Godeps/_workspace
USER_GOPATH := $(GOPATH)
export GOPATH = $(GODEPS):$(USER_GOPATH)

DEV_DEPS = github.com/uber/tchannel-go/thrift/thrift-gen \
		   github.com/vektra/mockery/... \
		   github.com/tools/godep

# Automatically gather packages
PKGS = $(shell find . -type d -maxdepth 3 \
	! -path '*/.git*' \
	! -path '*/_*' \
	! -path '*/Godeps*' \
	! -path '*/test*' \
	! -path '*/examples*' \
)

out:	test

clean:
	rm -f testpop

clean-mocks:
	rm -f test/mocks/*.go forward/mock_*.go
	rm -rf test/thrift/pingpong/

lint:
	@:>/tmp/lint.log

	@echo -e "\033[0;32mRunning golint...\033[0m"
	-golint ./... | grep -Ev '(test|examples)/' | tee -a /tmp/lint.log

	@echo -e "\033[0;32mRunning go vet...\033[0m"
	@for pkg in $(PKGS); do \
		{ \
			 find $$pkg -mindepth 1 -maxdepth 1 -name '*.go' \
				| xargs go tool vet 2>&1 ; \
		} | tee -a /tmp/lint.log ; \
	done;

	@[ ! -s /tmp/lint.log ]

mocks:
	test/gen-testfiles

setup:
	GOPATH=$(USER_GOPATH) go get -u -v $(DEV_DEPS)
	@if ! which thrift | grep -q /; then \
		echo "thrift not in PATH. (brew install thrift?)" >&2; \
 		exit 1; \
	fi

test:	test-unit test-integration

test-integration:
	test/run-integration-tests

test-unit:
	go generate ./...
	test/go-test-prettify ./...

testpop:	clean
	go build scripts/testpop/testpop.go
