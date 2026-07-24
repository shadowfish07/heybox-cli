.PHONY: build install test check clean

build:
	mkdir -p bin
	go build -trimpath -o bin/heybox ./cmd/heybox

install:
	go install ./cmd/heybox

test:
	go test ./...

check:
	go test ./...
	go vet ./...

clean:
	rm -f bin/heybox
