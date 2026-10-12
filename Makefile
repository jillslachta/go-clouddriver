GOOGLE_APPLICATION_CREDENTIALS ?= test/credentials/test-keyfile.json

default: all

all: clean lint build test

build:
	go build cmd/clouddriver/clouddriver.go

clean:
	go clean
	-rm ./clouddriver

lint:
	golangci-lint run

run: clean lint build test
	./clouddriver

test:
	GOOGLE_APPLICATION_CREDENTIALS=$(GOOGLE_APPLICATION_CREDENTIALS) ginkgo -r

tools:
	go install github.com/onsi/ginkgo/v2/ginkgo

vendor:
	go mod vendor

.PHONY: all clean build lint run test tools
