export PATH := $(shell pwd)/scripts/travis/thrift-release/linux-x86_64:$(PATH)
export PATH := $(shell pwd)/scripts/travis/thrift-gen-release/linux-x86_64:$(PATH)

.PHONY: clean clean-mocks testpop mocks out test test_ci

out:	test

clean:
	rm -f testpop

clean-mocks:
	rm -f test/mocks/*.go forward/mock_*.go
	rm -rf test/thrift/pingpong/

mocks:
	test/gen-testfiles

test:
	godep go generate ./...
	godep go test -v ./...

test_ci:
	go generate ./...
	go test -v ./...

testpop:	clean
	godep go build scripts/testpop/testpop.go
