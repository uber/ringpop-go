#!/bin/bash -x

set -e

cd "$(dirname "$0")"
wget -N https://github.com/prashantv/thrift/releases/download/p0.0.1/thrift-release.zip
unzip -o thrift-release.zip
