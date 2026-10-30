.PHONY: build auth run demo test check fmt fmt-check lint lint-actions lint-make docs-check audit-faults vuln-check cross notify-helper notify-release release

GOLANGCI_LINT ?= golangci-lint
GOFUMPT ?= go run mvdan.cc/gofumpt@$(shell cat .gofumpt-version)
ACTIONLINT ?= actionlint
CHECKMAKE ?= checkmake
GOVULNCHECK ?= go run golang.org/x/vuln/cmd/govulncheck@$(shell cat .govulncheck-version)
AUTH_ARGS ?= --email-code
RUN_ARGS ?=
THEME ?=
export THEME
MACOS_SIGN_IDENTITY ?=
MACOS_NOTARY_PROFILE ?=
MACOS_NOTARY_KEYCHAIN ?=
export MACOS_SIGN_IDENTITY MACOS_NOTARY_PROFILE MACOS_NOTARY_KEYCHAIN
VERSION ?= $(shell cat VERSION)
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
ifndef BUILD_DATE
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
endif
MODULE := github.com/cliffom/bambustand
LDFLAGS := -s -w -X $(MODULE)/internal/buildinfo.Version=$(VERSION) -X $(MODULE)/internal/buildinfo.Commit=$(COMMIT) -X $(MODULE)/internal/buildinfo.Date=$(BUILD_DATE)

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/bambustand ./cmd/bambustand

auth: build
	./bin/bambustand login $(AUTH_ARGS)

run: build
	./bin/bambustand $(RUN_ARGS) $${THEME:+--theme "$$THEME"}

demo: build
	./bin/bambustand --demo $(RUN_ARGS) $${THEME:+--theme "$$THEME"}

test:
	go test -race ./...

check:
	go vet ./...

vuln-check:
	$(GOVULNCHECK) ./...

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
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambustand-darwin-arm64 ./cmd/bambustand
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambustand-darwin-amd64 ./cmd/bambustand
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambustand-linux-arm64 ./cmd/bambustand
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags '$(LDFLAGS)' -o dist/bambustand-linux-amd64 ./cmd/bambustand

notify-helper:
	VERSION='$(VERSION)' bash tools/build-notify-helper.sh

notify-release:
	VERSION='$(VERSION)' bash tools/build-notify-helper.sh dist/notify-helper

release: cross notify-release
	python3 tools/prepare-macos-release.py prepare dist
	bash tools/package-release.sh '$(VERSION)'
