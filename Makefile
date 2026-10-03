.PHONY: build test vet install clean

build:
	go build -o bin/sshtree ./cmd/sshtree

test:
	go test ./...

vet:
	go vet ./...

install:
	go install ./cmd/sshtree

clean:
	rm -rf bin dist
