.PHONY: build test vet install clean

build:
	go build -o bin/sshtui ./cmd/sshtui

test:
	go test ./...

vet:
	go vet ./...

install:
	go install ./cmd/sshtui

clean:
	rm -rf bin dist
