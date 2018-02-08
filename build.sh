#/bin/bash

GOOS=linux GOARCH=amd64 go build -o dig-linux-amd64
GOOS=darwin GOARCH=amd64 go build -o dig-darwin-amd64
GOOS=windows GOARCH=amd64 go build -o dig-windows-amd64.exe
