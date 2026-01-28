BD_LITE_CMD ?= ./bd
BD_REF_CMD ?= $(shell which bd)

.PHONY: test test-unit test-e2e e2e-update build

test: pre-test-e2e
	BD_CMD=$(realpath $(BD_LITE_CMD)) go test ./...

test-unit:
	go test ./internal/... ./cmd/...

pre-test-e2e:
	@test -x "$(BD_LITE_CMD)" || (echo "error: bd binary not found at $(BD_LITE_CMD)" && echo "Run 'make build' first or set BD_LITE_CMD" && exit 1)

test-e2e: pre-test-e2e
	BD_CMD=$(realpath $(BD_LITE_CMD)) go test ./e2etests -v -count=1

e2e-update:
	@test -n "$(BD_REF_CMD)" || (echo "error: reference bd not found in PATH" && echo "Install bd or set BD_REF_CMD" && exit 1)
	@test -x "$(BD_REF_CMD)" || (echo "error: $(BD_REF_CMD) is not executable" && exit 1)
	BD_CMD=$(BD_REF_CMD) go test ./e2etests -update -v -count=1

build:
	go build -o bd ./cmd
