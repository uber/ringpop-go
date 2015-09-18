testpop:	clean
	godep go build scripts/testpop/testpop.go

clean:
	rm -f testpop

.PHONY: clean testpop
