.PHONY: build auth run demo test check fmt fmt-check lint lint-actions lint-make docs-check audit-faults cross release

GOLANGCI_LINT ?= golangci-lint
GOFUMPT ?= go run mvdan.cc/gofumpt@$(shell cat .gofumpt-version)
ACTIONLINT ?= actionlint
CHECKMAKE ?= checkmake
AUTH_ARGS ?= --email-code
RUN_ARGS ?=
VERSION ?= $(shell cat VERSION)
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
ifndef BUILD_DATE
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
endif
MODULE := github.com/cliffom/bambu-tui
LDFLAGS := -s -w -X $(MODULE)/internal/buildinfo.Version=$(VERSION) -X $(MODULE)/internal/buildinfo.Commit=$(COMMIT) -X $(MODULE)/internal/buildinfo.Date=$(BUILD_DATE)

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/bambu-tui ./cmd/bambu-tui

auth: build
	./bin/bambu-tui login $(AUTH_ARGS)

run: build
	./bin/bambu-tui $(RUN_ARGS)

demo: build
	./bin/bambu-tui --demo

test:
	go test -race ./...

check:
	go vet ./...

fmt:
	$(GOFUMPT) -w cmd internal

fmt-check:
	@set -e; files="$$($(GOFUMPT) -l cmd internal)"; \
	if [ -n "$$files" ]; then \
		printf 'Go files need formatting:\n%s\nRun make fmt to fix them.\n' "$$files"; \
		exit 1; \
	fi

lint:
	$(GOLANGCI_LINT) run

lint-actions:
	$(ACTIONLINT)

lint-make:
	$(CHECKMAKE) --config checkmake.ini Makefile

docs-check:
	python3 -m unittest discover -s tools -p 'test_check_docs.py'
	python3 tools/check-docs.py

audit-faults:
	BAMBU_AUDIT_UPSTREAM=1 go test ./internal/farm -run '^TestUpstreamFaultCatalogue$$' -count=1 -v

cross:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambu-tui-darwin-arm64 ./cmd/bambu-tui
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambu-tui-darwin-amd64 ./cmd/bambu-tui
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambu-tui-linux-arm64 ./cmd/bambu-tui
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambu-tui-linux-amd64 ./cmd/bambu-tui

release: cross
	bash tools/package-release.sh '$(VERSION)'
