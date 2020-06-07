#!/bin/bash

export GO111MODULE=on
export GOPROXY=https://goproxy.io

go build -o install/Vert

tar cfz vert_install.tgz install/

rm install/Vert