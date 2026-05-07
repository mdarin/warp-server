#!/bin/bash

start_time=$(date +%s)

GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 GO111MODULE=on go build -a -v -o warp-server cmd/app/* 2>&1 | while read -r line; do
    echo -e "\033[1;32m[Compile]\033[0m $line"
done

end_time=$(date +%s)
echo -e "\033[1;34m Build completed in $((end_time - start_time)) seconds\033[0m"
