#!/bin/bash

set -e

cd "$(dirname "$0")"
rm -rf thrift-gen.tar.gz
wget https://github.com/thanodnl/tchannel-go/releases/download/v0.1-alpha/thrift-gen.tar.gz
tar -xzf thrift-gen.tar.gz
mv release/ thrift-gen-release/