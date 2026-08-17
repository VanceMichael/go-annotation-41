GOTOOLCHAIN ?= local
export GOTOOLCHAIN

.PHONY: build test race vet fmt selfcheck clean

build:
	go build -trimpath -o bin/bioctl ./cmd/bioctl

test:
	go test ./... -count=1

race:
	go test -race ./... -count=1

vet:
	go vet ./...

fmt:
	gofmt -l .

selfcheck: build
	./bin/bioctl selfcheck

clean:
	rm -rf bin
