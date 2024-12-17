#!/bin/bash

GOOS=linux GOARCH=arm go build -o maiden.arm
make
make release
cd ./dist
ls

