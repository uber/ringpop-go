export PATH := $(shell pwd)/scripts/travis/thrift-release/linux-x86_64:$(PATH)
export PATH := $(shell pwd)/scripts/travis/thrift-gen-release/linux-x86_64:$(PATH)

.PHONY: clean clean-mocks testpop mocks out test

out:	test

clean:
	rm -f testpop

clean-mocks:
	rm -f test/mocks/*.go

mocks:
	test/gen-mocks

test:
	godep go generate ./...
	godep go test -v ./...

testpop:	clean
	godep go build scripts/testpop/testpop.go
