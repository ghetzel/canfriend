.PHONY: build ui

all: fmt deps build

deps:
	@go list github.com/mjibson/esc || go get github.com/mjibson/esc/...
	@go list golang.org/x/tools/cmd/goimports || go get golang.org/x/tools/cmd/goimports
	go generate -x
	go get . ./cli

fmt:
	goimports -w .
	go vet .

run:
	./bin/canfriend -L debug --ui-dir ui run

build: fmt
	go build -o bin/canfriend ./cli/
