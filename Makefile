.PHONY: build test check cross release
VERSION ?= $(shell cat VERSION)
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MODULE := github.com/cliffom/bambu-labs-tui
LDFLAGS := -s -w -X $(MODULE)/internal/buildinfo.Version=$(VERSION) -X $(MODULE)/internal/buildinfo.Commit=$(COMMIT) -X $(MODULE)/internal/buildinfo.Date=$(BUILD_DATE)
build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/bambu-tui ./cmd/bambu-tui
test:
	go test -race ./...
check:
	go vet ./...
cross:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambu-tui-darwin-arm64 ./cmd/bambu-tui
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambu-tui-darwin-amd64 ./cmd/bambu-tui
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambu-tui-linux-arm64 ./cmd/bambu-tui
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambu-tui-linux-amd64 ./cmd/bambu-tui
release: cross
	bash tools/package-release.sh '$(VERSION)'
