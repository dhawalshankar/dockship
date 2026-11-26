.PHONY: build test vet clean install run

BINARY_NAME=dockship
OUTPUT_DIR=bin

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(OUTPUT_DIR)/$(BINARY_NAME) ./cmd/dockship

test:
	go test -v -race ./...

vet:
	go vet ./...

clean:
	rm -rf $(OUTPUT_DIR) .gotmp .dockship_tmp

install:
	go install ./cmd/dockship
