DIST_DIR          := target/dist
TEST_DIR          := target/test
UNIT_COV_DIR      := ${TEST_DIR}/unit
UNIT_BIN_COV_DIR  := ${UNIT_COV_DIR}/cov/bin
UNIT_TXT_COV_DIR  := ${UNIT_COV_DIR}/cov/txt
UNIT_JUNIT_DIR    := ${UNIT_COV_DIR}/junit
APP_COV_DIR       := ${TEST_DIR}/application
APP_BIN_DIR       := ${APP_COV_DIR}/cov/bin
APP_TXT_COV_DIR   := ${APP_COV_DIR}/cov/txt
APP_JUNIT_DIR     := ${APP_COV_DIR}/junit
CMB_COV_DIR       := ${TEST_DIR}/combined
CMB_TXT_COV_DIR   := ${CMB_COV_DIR}/cov/txt

### Build variables
TARGET            = mystats
TARGET_DIR        = ${DIST_DIR}/${TARGET}
TARGET_BIN        = ${TARGET_DIR}/${TARGET}
TARGET_PKG        = .

.DEFAULT_GOAL := run/bin
.PHONY: help
help: ## Show help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_\/-]+:.*?##/ { printf "  \033[36m%-20s\033[0m  %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: bin
bin: ## Build binary
	mkdir -p ${TARGET_DIR}
	go build -o  ${TARGET_BIN} ${TARGET_PKG}

.PHONY: run/bin
run/bin: bin # telemetry-up db-up ## Run application binary
	${APP_ENV_VARS} \
	${TARGET_BIN} server --update=False

.PHONY: test
test: test/unit # test/app ## Run all tests and show coverage
	rm -rf ${CMB_TXT_COV_DIR}
	mkdir -p ${CMB_TXT_COV_DIR}
	go tool covdata textfmt -i=${APP_BIN_DIR},${UNIT_BIN_COV_DIR} -o ${CMB_TXT_COV_DIR}/cover.txt
	go tool cover -html=${CMB_TXT_COV_DIR}/cover.txt

.PHONY: test/unit
test/unit: ## Run unit tests
	rm -rf ${UNIT_BIN_COV_DIR} ${UNIT_TXT_COV_DIR} ${UNIT_JUNIT_DIR}
	mkdir -p ${UNIT_BIN_COV_DIR} ${UNIT_TXT_COV_DIR} ${UNIT_JUNIT_DIR}
	CGO_ENABLED=1 go tool gotest.tools/gotestsum --junitfile=${UNIT_JUNIT_DIR}/junit.xml -- -race -covermode=atomic -coverprofile=${UNIT_TXT_COV_DIR}/cover.txt ./... -test.gocoverdir=$(abspath ${UNIT_BIN_COV_DIR})

.PHONY: lint
lint: ## Run linter
	CGO_ENABLED=1 go tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint run ./...

.PHONY: clean
clean: ## Clean up environment
	rm -rf ./target
