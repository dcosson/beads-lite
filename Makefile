BD_LITE_CMD ?= ./bd
BD_REF_CMD ?= /opt/homebrew/bin/bd

.PHONY: test test-unit test-unit-coverage test-e2e bench-e2e bench-comparison-e2e e2e-update build check check-ci fmt fmt-check vet staticcheck deps

test: test-unit test-e2e

test-unit:
	go test -race ./internal/... ./cmd/... $(ARGS)

test-unit-coverage:
	go test -race -coverprofile=coverage.out ./internal/... ./cmd/... $(ARGS)
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-e2e: build
	@test -x "$(BD_LITE_CMD)" || (echo "error: bd binary not found at $(BD_LITE_CMD)" && echo "Run 'make build' first or set BD_LITE_CMD" && exit 1)
	@test -x "$(BD_LITE_CMD)" || (echo "error: $(BD_LITE_CMD) is not executable" && exit 1)
	BD_CMD=$(realpath $(BD_LITE_CMD)) BD_ACTOR=testactor GIT_AUTHOR_EMAIL=testactor@example.com go test ./e2etests/... $(ARGS)

test-concurrency:
	@test -x "$(BD_LITE_CMD)" || (echo "error: bd binary not found at $(BD_LITE_CMD)" && echo "Run 'make build' first or set BD_LITE_CMD" && exit 1)
	@test -x "$(BD_LITE_CMD)" || (echo "error: $(BD_LITE_CMD) is not executable" && exit 1)
	BD_CMD=$(realpath $(BD_LITE_CMD)) BD_ACTOR=testactor GIT_AUTHOR_EMAIL=testactor@example.com go test ./e2etests/concurrency -count=50 $(ARGS)

bench-e2e: build
	BD_CMD=$(realpath $(BD_LITE_CMD)) BD_ACTOR=testactor GIT_AUTHOR_EMAIL=testactor@example.com go test ./e2etests -run TestBenchmark -v -count=1 $(ARGS)

bench-comparison-e2e: build
	@test -n "$(BD_REF_CMD)" || (echo "error: reference bd not found in PATH" && exit 1)
	BD_CMD=$(realpath $(BD_LITE_CMD)) BD_REF_CMD=$(BD_REF_CMD) BD_ACTOR=testactor GIT_AUTHOR_EMAIL=testactor@example.com go test ./e2etests -run TestBenchmark -compare -v -count=1 $(ARGS)

update-e2e:
	@test -n "$(BD_REF_CMD)" || (echo "error: reference bd not found in PATH" && echo "Install bd or set BD_REF_CMD" && exit 1)
	@test -x "$(BD_REF_CMD)" || (echo "error: $(BD_REF_CMD) is not executable" && exit 1)
	BD_CMD=$(BD_REF_CMD) BD_ACTOR=testactor GIT_AUTHOR_EMAIL=testactor@example.com go test ./e2etests/reference -update -v -count=1 $(ARGS)

build:
	go build -o bd ./cmd

deps:
	go install honnef.co/go/tools/cmd/staticcheck@latest

check: fmt vet staticcheck

check-ci: fmt-check vet staticcheck

fmt:
	@echo "==> gofmt"
	gofmt -w .

fmt-check:
	@echo "==> gofmt (check)"
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "above files are not formatted" && exit 1)

vet:
	@echo "==> go vet"
	go vet ./...

staticcheck:
	@echo "==> staticcheck"
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...
