# silent build
V := @

BIN_DIR := ./bin
EXTENDER := $(BIN_DIR)/arch-sched

all: $(EXTENDER)

$(EXTENDER):
	@echo " GO" $@
	$(V)GOOS=linux go build -mod vendor \
		-ldflags "-X main.version=`(git describe --tags --dirty --always 2>/dev/null || echo "unknown") \
		| sed -e "s/^v//;s/-/_/g;s/_/-/;s/_/./g"`" \
		-o $(EXTENDER) ./cmd/arch-sched


GOBIN := $(shell go env GOPATH)/bin
LINTER := $(GOBIN)/golangci-lint
LINTER_VERSION := v1.17.1

.PHONY: linter-install
linter-install:
	@echo " INSTALL" $(LINTER) $(LINTER_VERSION)
	$(V)curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOBIN) $(LINTER_VERSION)

.PHONY: lint
lint:
	$(V) [ ! -x $(LINTER) ] && \
	 echo 'Linter is not installed, run `make linter-install`' && \
	 exit 1 || true
	@echo " RUNNING LINTER"
	$(V)$(LINTER) run --config .golangci.local.yml

.PHONY: clean
clean:
	@echo " CLEAN"
	$(V)go clean -mod vendor
	$(V)rm -rf $(BIN_DIR)

.PHONY: test
test:
	$(V)GOOS=linux go test -mod vendor -v -coverpkg=./... -coverprofile=cover.out -race ./...

dep:
	$(V)go mod tidy
	$(V)go mod vendor

