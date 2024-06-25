#!/usr/bin/env bash

set -xe
cd $(dirname $0)

protoc --go_out=. --go-grpc_out=. *.proto
mv github.com/reyoung/rce/protocol/*.go .
rm -rf github.com
