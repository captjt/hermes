#!/bin/bash

parent=$(dirname $PWD)
gopath=$(dirname $parent)
export GOPATH=$HOME/go:$gopath
echo "Your GOPATH is now set to '$gopath'"
echo "Type 'go env' to see your environment variables"
