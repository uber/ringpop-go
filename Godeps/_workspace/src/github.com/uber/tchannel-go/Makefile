GODEPS := $(shell pwd)/Godeps/_workspace
OLDGOPATH := $(GOPATH)
PATH := $(GODEPS)/bin:$(PATH)
EXAMPLES=./examples/bench/server ./examples/bench/client ./examples/ping ./examples/thrift ./examples/hyperbahn/echo-server
PKGS := . ./json ./hyperbahn ./thrift ./typed ./trace $(EXAMPLES)
TEST_ARG ?= -race -v -timeout 2m
BUILD := ./build
THRIFT_GEN_RELEASE := ./thrift-gen-release
THRIFT_GEN_RELEASE_LINUX := $(THRIFT_GEN_RELEASE)/linux-x86_64
THRIFT_GEN_RELEASE_DARWIN := $(THRIFT_GEN_RELEASE)/darwin-x86_64
SRCS := $(foreach pkg,$(PKGS),$(wildcard $(pkg)/*.go))
export GOPATH = $(GODEPS):$(OLDGOPATH)

PLATFORM := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
THRIFT_REL := ./scripts/travis/thrift-release/$(PLATFORM)-$(ARCH)

export PATH := $(realpath $(THRIFT_REL)):$(PATH)


# Separate packages that use testutils and don't, since they can have different flags.
# This is especially useful for timeoutMultiplier and connectionLog
TESTUTILS_TEST_PKGS := . hyperbahn testutils http json thrift pprof trace
NO_TESTUTILS_PKGS := stats thrift/thrift-gen tnet typed

# Cross language test args
TEST_HOST=127.0.0.1
TEST_PORT=0

all: test examples

packages_test:
	go list -json ./... | jq -r '. | select ((.TestGoFiles | length) > 0)  | .ImportPath'

setup:
	mkdir -p $(BUILD)
	mkdir -p $(BUILD)/examples
	mkdir -p $(THRIFT_GEN_RELEASE_LINUX)
	mkdir -p $(THRIFT_GEN_RELEASE_DARWIN)

get_thrift:
	scripts/travis/get-thrift.sh

install:
	GOPATH=$(GODEPS) go get github.com/tools/godep
	GOPATH=$(GODEPS) go get github.com/golang/lint
	GOPATH=$(GODEPS) godep restore -v

install_ci: get_thrift install
	go get -u github.com/mattn/goveralls

help:
	@egrep "^# target:" [Mm]akefile | sort -

clean:
	echo Cleaning build artifacts...
	go clean
	rm -rf $(BUILD) $(THRIFT_GEN_RELEASE)
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
	@echo Testing packages:
	go test $(addprefix github.com/uber/tchannel-go/,$(NO_TESTUTILS_PKGS)) $(TEST_ARG) -parallel=4
	go test $(addprefix github.com/uber/tchannel-go/,$(TESTUTILS_TEST_PKGS)) $(TEST_ARG) -timeoutMultiplier 10 -parallel=4
	@echo Running frame pool tests
	go test -run TestFramesReleased -stressTest $(TEST_ARG) -timeoutMultiplier 10

benchmark: clean setup
	echo Running benchmarks:
	go test $(PKGS) -bench=. -parallel=4

cover_profile: clean setup
	@echo Testing packages:
	mkdir -p $(BUILD)
	go test ./ $(TEST_ARG) -coverprofile=$(BUILD)/coverage.out

cover: cover_profile
	go tool cover -html=$(BUILD)/coverage.out

cover_ci: cover_profile
	goveralls -coverprofile=$(BUILD)/coverage.out -service=travis-ci


FILTER := grep -v -e '_string.go' -e '/gen-go/' -e '/mocks/'
lint:
	@echo "Running golint"
	-golint ./... | $(FILTER) | tee lint.log
	@echo "Running go vet"
	-go tool vet $(PKGS) | tee -a lint.log
	@echo "Checking gofmt"
	-gofmt -d . | tee -a lint.log
	@[ ! -s lint.log ]

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
	$(BUILD)/thrift-gen --generateThrift --inputFile thrift/test.thrift --outputDir thrift/gen-go/
	$(BUILD)/thrift-gen --generateThrift --inputFile examples/keyvalue/keyvalue.thrift --outputDir examples/keyvalue/gen-go
	$(BUILD)/thrift-gen --generateThrift --inputFile examples/thrift/test.thrift --outputDir examples/thrift/gen-go
	$(BUILD)/thrift-gen --generateThrift --inputFile hyperbahn/hyperbahn.thrift --outputDir hyperbahn/gen-go
	rm -rf trace/thrift/gen-go/tcollector && $(BUILD)/thrift-gen --generateThrift --inputFile trace/tcollector.thrift --outputDir trace/thrift/gen-go/

release_thrift_gen: clean setup
	GOOS=linux GOARCH=amd64 godep go build -o $(THRIFT_GEN_RELEASE_LINUX)/thrift-gen ./thrift/thrift-gen
	GOOS=darwin GOARCH=amd64 godep go build -o $(THRIFT_GEN_RELEASE_DARWIN)/thrift-gen ./thrift/thrift-gen
	tar -czf thrift-gen-release.tar.gz $(THRIFT_GEN_RELEASE)
	mv thrift-gen-release.tar.gz $(THRIFT_GEN_RELEASE)/

.PHONY: all help clean fmt format get_thrift install install_ci release_thrift_gen packages_test test test_ci lint
.SILENT: all help clean fmt format test lint
