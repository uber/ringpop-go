GODEPS := $(shell pwd)/Godeps/_workspace
OLDGOPATH := $(GOPATH)
PATH := $(GODEPS)/bin:$(PATH)
EXAMPLES=./examples/bench/server ./examples/bench/client ./examples/ping ./examples/thrift ./examples/hyperbahn/echo-server
PKGS := . ./json ./hyperbahn ./thrift ./typed ./trace $(EXAMPLES)
TEST_PKGS_NOPREFIX := $(shell go list ./... | sed -e 's/^.*uber\/tchannel-go//')
TEST_PKGS := $(addprefix github.com/uber/tchannel-go,$(TEST_PKGS_NOPREFIX))
BUILD := ./build
SRCS := $(foreach pkg,$(PKGS),$(wildcard $(pkg)/*.go))
export GOPATH = $(GODEPS):$(OLDGOPATH)
export PATH := $(realpath ./scripts/travis/thrift-release/linux-x86_64):$(PATH)

# Cross language test args
TEST_HOST=127.0.0.1
TEST_PORT=0

all: test examples

setup:
	mkdir -p $(BUILD)
	mkdir -p $(BUILD)/examples

get_thrift:
	scripts/travis/get-thrift.sh

install:
	GOPATH=$(GODEPS) go get github.com/tools/godep
	GOPATH=$(GODEPS) godep restore

install_ci: get_thrift install

help:
	@egrep "^# target:" [Mm]akefile | sort -

clean:
	echo Cleaning build artifacts...
	go clean
	rm -rf $(BUILD)
	echo

fmt format:
	echo Formatting Packages...
	go fmt $(PKGS)
	echo

godep:
	rm -rf Godeps
	godep save ./...

test_ci: test

test: clean setup
	echo Testing packages:
	go test $(TEST_PKGS) $(TEST_ARG) -parallel=4

benchmark: clean setup
	echo Running benchmarks:
	go test $(PKGS) -bench=. -parallel=4

cover: clean setup
	echo Testing packages:
	mkdir -p $(BUILD)
	go test ./ $(TEST_ARG)  -coverprofile=$(BUILD)/coverage.out
	go tool cover -html=$(BUILD)/coverage.out

vet:
	echo Vetting packages for potential issues...
	go tool vet $(PKGS)
	echo

thrift_example: thrift_gen
	go build -o $(BUILD)/examples/thrift       ./examples/thrift/main.go

test_server:
	./build/examples/test_server --host ${TEST_HOST} --port ${TEST_PORT}

examples: clean setup thrift_example
	echo Building examples...
	mkdir -p $(BUILD)/examples/ping $(BUILD)/examples/bench
	go build -o $(BUILD)/examples/ping/pong    ./examples/ping/main.go
	go build -o $(BUILD)/examples/hyperbahn/echo-server    ./examples/hyperbahn/echo-server/main.go
	go build -o $(BUILD)/examples/bench/server ./examples/bench/server
	go build -o $(BUILD)/examples/bench/client ./examples/bench/client
	go build -o $(BUILD)/examples/bench/runner ./examples/bench/runner.go
	go build -o $(BUILD)/examples/test_server ./examples/test_server

thrift_gen:
	go build -o $(BUILD)/thrift-gen ./thrift/thrift-gen
	$(BUILD)/thrift-gen --generateThrift --inputFile thrift/test.thrift
	$(BUILD)/thrift-gen --generateThrift --inputFile examples/keyvalue/keyvalue.thrift
	$(BUILD)/thrift-gen --generateThrift --inputFile examples/thrift/test.thrift
	rm -rf trace/thrift/gen-go/tcollector && $(BUILD)/thrift-gen --generateThrift --inputFile trace/tcollector.thrift && cd trace && mv gen-go/* thrift/gen-go/

.PHONY: all help clean fmt format get_thrift install install_ci test test_ci vet
.SILENT: all help clean fmt format test vet
