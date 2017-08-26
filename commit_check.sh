#!/bin/bash
 
cp -r ../common Godeps/_workspace/src/github-beta.huawei.com/hipaas
 
echo "!!! golint begin ..."
golint ./...
echo "!!! golint end"
echo
 
echo "!!! go vet begin ..."
go vet ./...
echo "!!! go vet end"
echo
 
echo "!!! go fmt begin ..."
go fmt ./...
echo "!!! go fmt end"
echo
 
echo "!!! go test begin ..."
go test ./...
echo "!!! go test end"
echo
 
echo "!!! go build begin ..."
godep go build
echo "!!! go build end"
echo
