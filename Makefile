BD_LITE_CMD ?= ./bd
BD_REF_CMD ?= $(shell which bd)

.PHONY: test test-unit test-e2e e2e-update build

test: test-unit test-e2e

test-unit:
	go test -race ./internal/... ./cmd/... $(ARGS)

test-e2e: build
	@test -x "$(BD_LITE_CMD)" || (echo "error: bd binary not found at $(BD_LITE_CMD)" && echo "Run 'make build' first or set BD_LITE_CMD" && exit 1)
	@test -x "$(BD_LITE_CMD)" || (echo "error: $(BD_LITE_CMD) is not executable" && exit 1)
	BD_CMD=$(realpath $(BD_LITE_CMD)) BD_ACTOR=testactor GIT_AUTHOR_EMAIL=testactor@example.com go test ./e2etests/... $(ARGS)

update-e2e:
	@test -n "$(BD_REF_CMD)" || (echo "error: reference bd not found in PATH" && echo "Install bd or set BD_REF_CMD" && exit 1)
	@test -x "$(BD_REF_CMD)" || (echo "error: $(BD_REF_CMD) is not executable" && exit 1)
	BD_CMD=$(BD_REF_CMD) BD_ACTOR=testactor GIT_AUTHOR_EMAIL=testactor@example.com go test ./e2etests/reference -update -v -count=1 $(ARGS)

build:
	go build -o bd ./cmd
