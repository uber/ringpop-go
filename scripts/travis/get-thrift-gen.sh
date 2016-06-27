#!/bin/bash

set -e

cd "$(dirname "$0")"
rm -rf thrift-gen-release.tar.gz
wget https://github.com/uber/tchannel-go/releases/download/v0.01/thrift-gen-release.tar.gz
tar -xzf thrift-gen-release.tar.gz