.PHONY: clean clean-mocks testpop mocks out test

out:	test

clean:
	rm -f testpop

clean-mocks:
	rm -f test/mocks/*.go

mocks:
	test/gen-mocks

test:
	go test ./...

testpop:	clean
	godep go build scripts/testpop/testpop.go
