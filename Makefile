VERSION := `git describe --tags`
SOURCES ?= $(shell find . -name "*.go" -type f)
BINARY_NAME = wgmesh
NOW = `date +"%Y-%m-%d_%H-%M-%S"`
MAIN_GO_PATH=wgmesh.go

all: clean gen lint build 

.PHONY: build
build: gen
	CGO_ENABLED=0 GOOS=linux go build -i -v -o dist/${BINARY_NAME} ${MAIN_GO_PATH}

.PHONY: staticcheck
staticcheck:
	staticcheck ./...

.PHONY: lint
lint:
	@for file in ${SOURCES} ;  do \
		golint $$file ; \
	done

.PHONY: gen
gen:
	(cd meshservice ; protoc --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative  --go_out=. --go-grpc_out=. meshservice.proto agent.proto)

.PHONY: release
release: gen
	goreleaser --snapshot --rm-dist

.PHONY: clean
clean:
	rm -rf dist/*
	rm -f cover.out
	go clean -testcache
