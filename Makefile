GO111MODULE := on
DOCKER_TAG := $(or ${GITHUB_TAG_NAME}, latest)

.PHONY: all
all: proto server client

.PHONY: clean
clean: 
	rm -f api/v1/*pb.go bin/*

.PHONY: proto
proto:
	protoc -I api/ api/v1/*.proto --go_out=plugins=grpc:api

.PHONY: server
server:
	go build -tags netgo -o bin/server server/main.go
	strip bin/server

.PHONY: client
client:
	go build -tags netgo -o bin/client client/main.go
	strip bin/client

.PHONY: run-server
run-server:
	GODEBUG=http2debug=2 bin/server -port 50052