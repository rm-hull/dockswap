BINARY_NAME=dockswap

.PHONY: build clean test release

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) main.go

clean:
	@echo "Cleaning..."
	rm -rf bin/

test:
	go test ./...

release:
	GOOS=linux   GOARCH=amd64 go build -o bin/$(BINARY_NAME)-linux-amd64   main.go
	GOOS=linux   GOARCH=arm64 go build -o bin/$(BINARY_NAME)-linux-arm64   main.go
	GOOS=darwin  GOARCH=amd64 go build -o bin/$(BINARY_NAME)-darwin-amd64  main.go
	GOOS=darwin  GOARCH=arm64 go build -o bin/$(BINARY_NAME)-darwin-arm64  main.go
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY_NAME)-windows-amd64.exe main.go
