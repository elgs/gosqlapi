#!/usr/bin/env bash

# egrep -v "^go \d+\.\d+\.\d+$" go.mod > go.mod.tmp
# mv go.mod.tmp go.mod
# go mod tidy

GOPROXY=direct go get -u -t
go mod tidy
