.PHONY: build run clean test

build:
	go build -ldflags="-s -w" -o hnews .

run:
	go run .

clean:
	rm -f hnews

test:
	go test ./...
