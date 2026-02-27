
# removes static build artifacts
.PHONY: clean
clean:
	@echo "--------------> Running 'make clean'"
	@rm -rf binaries tmp
	rm -f *.tgz

# Define the `build` target
API ?= trcli

.PHONY: build
build:
	go build -o  binaries/$(API) ./cmd/trcli/*.go

.PHONY: docker
docker:
	docker build -t transfer .

.PHONY: test
test:
	@PATH="$$(go env GOPATH)/bin:$$PATH" gotestsum --version >/dev/null 2>&1 || { echo "gotestsum is required. Install: go install gotest.tools/gotestsum@latest"; exit 1; }
	@set -euo pipefail; \
	rerun_flag=""; \
	if [[ "$(RERUN_FAILS)" == "1" ]]; then \
		rerun_flag="--rerun-fails=2"; \
	fi; \
	LOG_LEVEL=ERROR YT_LOG_LEVEL=ERROR \
	PATH="$$(go env GOPATH)/bin:$$PATH" USE_TESTCONTAINERS=1 gotestsum $$rerun_flag --format $(GOTESTSUM_FORMAT) --packages="./cmd/..." -- -timeout=30m


# Define variables for the suite group, path, and name with defaults
SUITE_GROUP ?= 'tests/e2e'
SUITE_PATH ?= 'pg2ch'
SUITE_NAME ?= 'e2e-pg2ch'
GO_TEST_ARGS ?= -timeout=15m
SHELL := /bin/bash
GOTESTSUM_FORMAT ?= standard-quiet
ifeq ($(GITHUB_ACTIONS),true)
GOTESTSUM_FORMAT = github-actions
endif
MATRIX_CONTRACT ?= tests/e2e/matrix/core2ch.yaml
MATRIX_TOOL ?= go run ./tools/testmatrix
MATRIX_REPORT ?= tests/e2e/matrix/coverage_report.md
MATRIX_TEST_GO_ARGS ?= -count=1 -timeout=20m
CDC_SUITE_MANIFEST ?= tests/e2e/matrix/cdc_local_suite.yaml
CDC_OPTIONAL_SUITE_MANIFEST ?= tests/e2e/matrix/cdc_optional_suite.yaml
CDC_GO_TEST_ARGS ?= -timeout=20m
TEST_STATE_DIR ?= .teststate
TEST_STATE_WAVES_DIR ?= $(TEST_STATE_DIR)/waves
TEST_STATE_OPTIONAL_WAVES_DIR ?= $(TEST_STATE_DIR)/waves-optional
TEST_STATE_MATRIX_DIR ?= $(TEST_STATE_DIR)/matrix
FORCE ?= 0
RERUN_FAILS ?= 1

SUPPORTED_FLOW_DBS := pg2ch mysql2ch mongo2ch
SUPPORTED_COMPONENT_DBS := postgres mysql mongo
SUPPORTED_STREAM_FLOW_DBS := kafka2ch
SUPPORTED_OPTIONAL_FLOW_DBS := kafka2ch eventhub2ch kinesis2ch airbyte2ch oracle2ch ch2ch
SUPPORTED_LAYERS := storage canon e2e evolution resume large
SUPPORTED_SOURCE_VARIANTS := \
	postgres/17 postgres/18 \
	mysql/mysql84 mysql/mariadb118 \
	mongo/6 mongo/7 \
	kafka/confluent75 kafka/redpanda24
RESUME_TEST_PATTERN ?= ResumeFromCoordinator|Resume
LAYER ?= e2e
DB ?= pg2ch
SOURCE_VARIANT ?=
MATRIX_FAMILY ?= postgres
MATRIX_CORE_LAYERS ?= e2e evolution large
MATRIX_GO_TEST_ARGS ?= -count=1 -timeout=15m
KAFKA_MATRIX_LAYERS ?= e2e evolution large
CDC_WAVES := providers storage-canon e2e evolution resume large
WAVE_TARGETS := $(addprefix $(TEST_STATE_WAVES_DIR)/,$(addsuffix .ok,$(CDC_WAVES)))
CDC_WAVE_SHARED_PATHS := library pkg vendor_patched tools/testmatrix
WAVE_PATHS_providers := tests/helpers tests/tcrecipes
WAVE_PATHS_storage-canon := tests/storage tests/canon
WAVE_PATHS_e2e := tests/e2e
WAVE_PATHS_evolution := tests/evolution
WAVE_PATHS_resume := tests/resume
WAVE_PATHS_large := tests/large
CDC_OPTIONAL_WAVES := optional-queues optional-connectors optional-clickhouse-source
OPTIONAL_WAVE_TARGETS := $(addprefix $(TEST_STATE_OPTIONAL_WAVES_DIR)/,$(addsuffix .ok,$(CDC_OPTIONAL_WAVES)))
CDC_OPTIONAL_WAVE_SHARED_PATHS := library pkg vendor_patched tools/testmatrix
OPTIONAL_WAVE_PATHS_optional-queues := tests/e2e/kafka2ch tests/e2e/eventhub2ch tests/e2e/kinesis2ch tests/tcrecipes
OPTIONAL_WAVE_PATHS_optional-connectors := tests/e2e/airbyte2ch tests/e2e/oracle2ch tests/tcrecipes
OPTIONAL_WAVE_PATHS_optional-clickhouse-source := tests/e2e/ch2ch

define LIST_TRACKED_FILES
$(strip $(shell \
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then \
	git ls-files -- $(1) 2>/dev/null; \
else \
	for p in $(1); do \
		if [ -d "$$p" ]; then find "$$p" -type f; fi; \
	done; \
fi))
endef

COMMON_WAVE_DEPS := Makefile go.mod go.sum $(CDC_SUITE_MANIFEST) $(MATRIX_CONTRACT) $(call LIST_TRACKED_FILES,$(CDC_WAVE_SHARED_PATHS))
WAVE_DEPS_providers := $(call LIST_TRACKED_FILES,$(WAVE_PATHS_providers))
WAVE_DEPS_storage-canon := $(call LIST_TRACKED_FILES,$(WAVE_PATHS_storage-canon))
WAVE_DEPS_e2e := $(call LIST_TRACKED_FILES,$(WAVE_PATHS_e2e))
WAVE_DEPS_evolution := $(call LIST_TRACKED_FILES,$(WAVE_PATHS_evolution))
WAVE_DEPS_resume := $(call LIST_TRACKED_FILES,$(WAVE_PATHS_resume))
WAVE_DEPS_large := $(call LIST_TRACKED_FILES,$(WAVE_PATHS_large))
COMMON_OPTIONAL_WAVE_DEPS := Makefile go.mod go.sum $(CDC_OPTIONAL_SUITE_MANIFEST) $(call LIST_TRACKED_FILES,$(CDC_OPTIONAL_WAVE_SHARED_PATHS))
OPTIONAL_WAVE_DEPS_optional-queues := $(call LIST_TRACKED_FILES,$(OPTIONAL_WAVE_PATHS_optional-queues))
OPTIONAL_WAVE_DEPS_optional-connectors := $(call LIST_TRACKED_FILES,$(OPTIONAL_WAVE_PATHS_optional-connectors))
OPTIONAL_WAVE_DEPS_optional-clickhouse-source := $(call LIST_TRACKED_FILES,$(OPTIONAL_WAVE_PATHS_optional-clickhouse-source))
MATRIX_CACHE_SHARED_PATHS := library pkg vendor_patched tools/testmatrix tests/e2e tests/evolution tests/large
COMMON_MATRIX_DEPS := Makefile go.mod go.sum $(CDC_SUITE_MANIFEST) $(MATRIX_CONTRACT) $(call LIST_TRACKED_FILES,$(MATRIX_CACHE_SHARED_PATHS))

# Define the `run-tests` target
.PHONY: run-tests
run-tests:
	@echo "Running $(SUITE_GROUP) suite $(SUITE_NAME)"
	@PATH="$$(go env GOPATH)/bin:$$PATH" gotestsum --version >/dev/null 2>&1 || { echo "gotestsum is required. Install: go install gotest.tools/gotestsum@latest"; exit 1; }
	@export RECIPE_CLICKHOUSE_BIN=clickhouse; \
	export PATH="$$(go env GOPATH)/bin:$$PATH"; \
	export USE_TESTCONTAINERS=1; \
	export YA_TEST_RUNNER=1; \
	export YT_PROXY=localhost:8180; \
	export TEST_DEPS_BINARY_PATH=binaries; \
	export LOG_LEVEL=ERROR; \
	export YT_LOG_LEVEL=ERROR; \
	test_dirs="$$(find -L ./$(SUITE_GROUP)/$(SUITE_PATH) -type f -name '*_test.go' -exec dirname {} \; | sort -u)"; \
	if [[ -z "$$test_dirs" ]]; then \
	  echo "No Go test files found under ./$(SUITE_GROUP)/$(SUITE_PATH), skipping suite."; \
	  exit 0; \
	fi; \
	failed_dirs=""; \
	for dir in $$test_dirs; do \
	  echo "::group::$$dir"; \
	  echo "Running tests for directory: $$dir"; \
	  sanitized_dir=$$(echo "$$dir" | sed 's|/|_|g'); \
	  rerun_flag=""; \
	  if [[ "$(RERUN_FAILS)" == "1" ]]; then \
	    rerun_flag="--rerun-fails=2"; \
	  fi; \
	  if ! gotestsum \
	    --junitfile="reports/$(SUITE_NAME)_$$sanitized_dir.xml" \
	    --junitfile-project-name="$(SUITE_GROUP)" \
	    --junitfile-testsuite-name="short" \
	    $$rerun_flag \
	    --format $(GOTESTSUM_FORMAT) \
	    --packages="$$dir" \
	    -- $(GO_TEST_ARGS); then \
	    failed_dirs="$$failed_dirs $$dir"; \
	  fi; \
	  echo "::endgroup::"; \
		done; \
	if [[ -n "$$failed_dirs" ]]; then \
	  echo "Failed test directories:$$failed_dirs"; \
	  exit 1; \
	fi

.PHONY: run-go-packages
run-go-packages:
	@if [[ -z "$(PKG_PATTERN)" ]]; then \
		echo "PKG_PATTERN is required"; \
		exit 1; \
	fi
	@pkg_name="$(PKG_NAME)"; \
	if [[ -z "$$pkg_name" ]]; then \
		pkg_name="go-packages"; \
	fi; \
	pkg_go_test_args="$(PKG_GO_TEST_ARGS)"; \
	if [[ -z "$$pkg_go_test_args" ]]; then \
		pkg_go_test_args="$(CDC_GO_TEST_ARGS)"; \
	fi; \
	echo "Running package suite $$pkg_name ($$pkg_go_test_args)"; \
	PATH="$$(go env GOPATH)/bin:$$PATH"; \
	command -v gotestsum >/dev/null 2>&1 || { echo "gotestsum is required. Install: go install gotest.tools/gotestsum@latest"; exit 1; }; \
	export RECIPE_CLICKHOUSE_BIN=clickhouse; \
	export USE_TESTCONTAINERS=1; \
	export YA_TEST_RUNNER=1; \
	export TEST_DEPS_BINARY_PATH=binaries; \
	export LOG_LEVEL=ERROR; \
	export YT_LOG_LEVEL=ERROR; \
	sanitized_name="$$(echo "$$pkg_name" | sed 's|/|_|g')"; \
	rerun_flag=""; \
	if [[ "$(RERUN_FAILS)" == "1" ]]; then \
		rerun_flag="--rerun-fails=2"; \
	fi; \
	gotestsum \
		--junitfile="reports/$$sanitized_name.xml" \
		--junitfile-project-name="cdc-packages" \
		--junitfile-testsuite-name="short" \
		$$rerun_flag \
		--format $(GOTESTSUM_FORMAT) \
		--packages="$(PKG_PATTERN)" \
		-- $$pkg_go_test_args

.PHONY: test-list
test-list:
	@echo "Supported layers: $(SUPPORTED_LAYERS)"
	@echo "Supported flow DB aliases: $(SUPPORTED_FLOW_DBS)"
	@echo "Supported stream flow DB aliases: $(SUPPORTED_STREAM_FLOW_DBS)"
	@echo "Supported component DB names: $(SUPPORTED_COMPONENT_DBS)"
	@echo "Examples:"
	@echo "  make test-layer LAYER=e2e DB=pg2ch"
	@echo "  make test-layer-all LAYER=resume"
	@echo "  make test-db DB=mysql2ch"
	@echo "  make test-core"
	@echo "  make test-all-supported"
	@echo "  make test-source-variant SOURCE_VARIANT=postgres/18"
	@echo "  make test-layer LAYER=resume DB=kafka2ch"
	@echo "  make test-source-family MATRIX_FAMILY=mysql"
	@echo "  make test-source-matrix"
	@echo "  make test-matrix-gap-report"
	@echo "  make test-matrix-core"
	@echo "  make test-cdc-list"
	@echo "  make test-cdc-verify"
	@echo "  make test-cdc-wave WAVE=providers"
	@echo "  make test-cdc-wave WAVE=providers FORCE=1"
	@echo "  make test-cdc-matrix"
	@echo "  make test-cdc-matrix SOURCE_VARIANT=postgres/18"
	@echo "  make test-cdc-full"
	@echo "  make test-cdc-optional-list"
	@echo "  make test-cdc-optional-verify"
	@echo "  make test-cdc-optional-wave WAVE=optional-queues"
	@echo "  make test-cdc-optional"
	@echo "  make test-layer-optional DB=kinesis2ch"
	@echo "  make test-state-list"
	@echo "  make test-state-clear WAVE=providers"
	@echo "  make test-state-clear-all"
	@echo "  make test-state-optional-list"
	@echo "  make test-state-optional-clear WAVE=optional-queues"
	@echo "  make test-state-optional-clear-all"
	@echo "  make test-state-matrix-list"
	@echo "  make test-state-matrix-clear SOURCE_VARIANT=postgres/18"
	@echo "  make test-state-matrix-clear-all"

.PHONY: test-cdc-list
test-cdc-list:
	@$(MATRIX_TOOL) suite --manifest "$(CDC_SUITE_MANIFEST)" list

.PHONY: test-cdc-verify
test-cdc-verify:
	@$(MATRIX_TOOL) suite --manifest "$(CDC_SUITE_MANIFEST)" verify

.PHONY: test-cdc-optional-list
test-cdc-optional-list:
	@$(MATRIX_TOOL) suite --manifest "$(CDC_OPTIONAL_SUITE_MANIFEST)" list

.PHONY: test-cdc-optional-verify
test-cdc-optional-verify:
	@$(MATRIX_TOOL) suite --manifest "$(CDC_OPTIONAL_SUITE_MANIFEST)" verify

.PHONY: test-cdc-wave
test-cdc-wave:
	@if [[ -z "$(WAVE)" ]]; then \
		echo "WAVE is required (example: providers)"; \
		exit 1; \
	fi
	@set -euo pipefail; \
	wave="$(WAVE)"; \
	if [[ " $(CDC_WAVES) " != *" $$wave "* ]]; then \
		echo "Unsupported WAVE '$$wave'. Use one of: $(CDC_WAVES)"; \
		exit 1; \
	fi; \
	target="$(TEST_STATE_WAVES_DIR)/$$wave.ok"; \
	if [[ "$(FORCE)" == "1" ]]; then \
		rm -f "$$target"; \
	else \
		if [[ -f "$$target" ]]; then \
			echo "SKIP (cached): $$wave"; \
			exit 0; \
		fi; \
	fi; \
	$(MAKE) test-cdc-wave-run WAVE="$$wave"

.PHONY: test-cdc-wave-run
test-cdc-wave-run:
	@if [[ -z "$(WAVE)" ]]; then \
		echo "WAVE is required (example: providers)"; \
		exit 1; \
	fi
	@set -euo pipefail; \
	wave="$(WAVE)"; \
	run_wave_items() { \
		local coordinator_backend="$$1"; \
		local item_count=0; \
		while IFS=$$'\t' read -r kind a b c d; do \
			[[ -z "$$kind" ]] && continue; \
			item_count=$$((item_count + 1)); \
			case "$$kind" in \
				SUITE) \
					suite_group="$$a"; \
					suite_path="$$b"; \
					suite_name="$$c"; \
					suite_go_test_args="$$d"; \
					if [[ -z "$$suite_go_test_args" ]]; then \
						suite_go_test_args="$(GO_TEST_ARGS)"; \
					fi; \
					if [[ -n "$$coordinator_backend" ]]; then \
						echo "=== wave=$$wave backend=$$coordinator_backend suite=$$suite_group/$$suite_path ==="; \
						COORDINATOR_BACKEND="$$coordinator_backend" $(MAKE) run-tests SUITE_GROUP="$$suite_group" SUITE_PATH="$$suite_path" SUITE_NAME="$$suite_name" GO_TEST_ARGS="$$suite_go_test_args"; \
					else \
						echo "=== wave=$$wave suite=$$suite_group/$$suite_path ==="; \
						$(MAKE) run-tests SUITE_GROUP="$$suite_group" SUITE_PATH="$$suite_path" SUITE_NAME="$$suite_name" GO_TEST_ARGS="$$suite_go_test_args"; \
					fi; \
					;; \
				PKG) \
					pkg_pattern="$$a"; \
					pkg_name="$$b"; \
					pkg_go_test_args="$$c"; \
					echo "=== wave=$$wave package=$$pkg_pattern ==="; \
					$(MAKE) run-go-packages PKG_PATTERN="$$pkg_pattern" PKG_NAME="$$pkg_name" PKG_GO_TEST_ARGS="$$pkg_go_test_args"; \
					;; \
				*) \
					echo "Unknown item kind from manifest: $$kind"; \
					exit 1; \
					;; \
			esac; \
		done < <($(MATRIX_TOOL) suite --manifest "$(CDC_SUITE_MANIFEST)" emit-wave --wave "$$wave"); \
		if [[ "$$item_count" -eq 0 ]]; then \
			echo "No runnable items found for wave '$$wave'"; \
			exit 1; \
		fi; \
	}; \
	run_wave_items ""; \
	mkdir -p "$(TEST_STATE_WAVES_DIR)"; \
	date -u +"%Y-%m-%dT%H:%M:%SZ" > "$(TEST_STATE_WAVES_DIR)/$$wave.ok"

$(TEST_STATE_WAVES_DIR):
	@mkdir -p "$@"

.SECONDEXPANSION:
$(TEST_STATE_WAVES_DIR)/%.ok: $$(COMMON_WAVE_DEPS) $$(WAVE_DEPS_$$*) | $(TEST_STATE_WAVES_DIR)
	@set -euo pipefail; \
	wave="$*"; \
	run_wave_items() { \
		local coordinator_backend="$$1"; \
		local item_count=0; \
		while IFS=$$'\t' read -r kind a b c d; do \
			[[ -z "$$kind" ]] && continue; \
			item_count=$$((item_count + 1)); \
			case "$$kind" in \
				SUITE) \
					suite_group="$$a"; \
					suite_path="$$b"; \
					suite_name="$$c"; \
					suite_go_test_args="$$d"; \
					if [[ -z "$$suite_go_test_args" ]]; then \
						suite_go_test_args="$(GO_TEST_ARGS)"; \
					fi; \
					if [[ -n "$$coordinator_backend" ]]; then \
						echo "=== wave=$$wave backend=$$coordinator_backend suite=$$suite_group/$$suite_path ==="; \
						COORDINATOR_BACKEND="$$coordinator_backend" $(MAKE) run-tests SUITE_GROUP="$$suite_group" SUITE_PATH="$$suite_path" SUITE_NAME="$$suite_name" GO_TEST_ARGS="$$suite_go_test_args"; \
					else \
						echo "=== wave=$$wave suite=$$suite_group/$$suite_path ==="; \
						$(MAKE) run-tests SUITE_GROUP="$$suite_group" SUITE_PATH="$$suite_path" SUITE_NAME="$$suite_name" GO_TEST_ARGS="$$suite_go_test_args"; \
					fi; \
					;; \
				PKG) \
					pkg_pattern="$$a"; \
					pkg_name="$$b"; \
					pkg_go_test_args="$$c"; \
					echo "=== wave=$$wave package=$$pkg_pattern ==="; \
					$(MAKE) run-go-packages PKG_PATTERN="$$pkg_pattern" PKG_NAME="$$pkg_name" PKG_GO_TEST_ARGS="$$pkg_go_test_args"; \
					;; \
				*) \
					echo "Unknown item kind from manifest: $$kind"; \
					exit 1; \
					;; \
			esac; \
		done < <($(MATRIX_TOOL) suite --manifest "$(CDC_SUITE_MANIFEST)" emit-wave --wave "$$wave"); \
		if [[ "$$item_count" -eq 0 ]]; then \
			echo "No runnable items found for wave '$$wave'"; \
			exit 1; \
		fi; \
	}; \
	run_wave_items ""; \
	date -u +"%Y-%m-%dT%H:%M:%SZ" > "$@"

.PHONY: test-cdc-matrix
test-cdc-matrix:
	@set -euo pipefail; \
	common_deps="$(COMMON_MATRIX_DEPS)"; \
	is_stale() { \
		local target="$$1"; \
		if [[ ! -f "$$target" ]]; then \
			return 0; \
		fi; \
		for dep in $$common_deps; do \
			if [[ -f "$$dep" && "$$dep" -nt "$$target" ]]; then \
				return 0; \
			fi; \
		done; \
		return 1; \
	}; \
	if [[ -n "$(SOURCE_VARIANT)" ]]; then \
		source_variant="$(SOURCE_VARIANT)"; \
		slug="$$(echo "$$source_variant" | sed 's|/|-|g')"; \
		target="$(TEST_STATE_MATRIX_DIR)/$$slug.ok"; \
		if [[ "$(FORCE)" == "1" ]]; then \
			rm -f "$$target"; \
		else \
			if ! is_stale "$$target"; then \
				echo "SKIP (cached): matrix $$source_variant"; \
				exit 0; \
			fi; \
		fi; \
		$(MAKE) test-cdc-matrix-run SOURCE_VARIANT="$$source_variant"; \
		exit 0; \
	fi; \
	while IFS= read -r source_variant; do \
		[[ -z "$$source_variant" ]] && continue; \
		slug="$$(echo "$$source_variant" | sed 's|/|-|g')"; \
		target="$(TEST_STATE_MATRIX_DIR)/$$slug.ok"; \
		if [[ "$(FORCE)" == "1" ]]; then \
			rm -f "$$target"; \
		else \
			if ! is_stale "$$target"; then \
				echo "SKIP (cached): matrix $$source_variant"; \
				continue; \
			fi; \
		fi; \
		$(MAKE) test-cdc-matrix-run SOURCE_VARIANT="$$source_variant"; \
	done < <($(MATRIX_TOOL) suite --manifest "$(CDC_SUITE_MANIFEST)" emit-matrix --scope all)

.PHONY: test-cdc-matrix-run
test-cdc-matrix-run:
	@if [[ -z "$(SOURCE_VARIANT)" ]]; then \
		echo "SOURCE_VARIANT is required (example: postgres/18)"; \
		exit 1; \
	fi
	@set -euo pipefail; \
	source_variant="$(SOURCE_VARIANT)"; \
	slug="$$(echo "$$source_variant" | sed 's|/|-|g')"; \
	echo "=== cdc-matrix variant=$$source_variant ==="; \
	SOURCE_VARIANT="$$source_variant" $(MAKE) test-source-variant; \
	mkdir -p "$(TEST_STATE_MATRIX_DIR)"; \
	date -u +"%Y-%m-%dT%H:%M:%SZ" > "$(TEST_STATE_MATRIX_DIR)/$$slug.ok"

$(TEST_STATE_MATRIX_DIR):
	@mkdir -p "$@"

.SECONDEXPANSION:
$(TEST_STATE_MATRIX_DIR)/%.ok: $$(COMMON_MATRIX_DEPS) | $(TEST_STATE_MATRIX_DIR)
	@set -euo pipefail; \
	source_variant="$(SOURCE_VARIANT)"; \
	if [[ -z "$$source_variant" ]]; then \
		echo "SOURCE_VARIANT is required to build matrix cache target"; \
		exit 1; \
	fi; \
	echo "=== cdc-matrix variant=$$source_variant ==="; \
	SOURCE_VARIANT="$$source_variant" $(MAKE) test-source-variant; \
	date -u +"%Y-%m-%dT%H:%M:%SZ" > "$@"

.PHONY: test-cdc-full
test-cdc-full:
	@set -euo pipefail; \
	$(MAKE) test-cdc-verify; \
	while IFS= read -r wave; do \
		[[ -z "$$wave" ]] && continue; \
		echo "=== cdc-full wave=$$wave ==="; \
		$(MAKE) test-cdc-wave WAVE="$$wave"; \
	done < <($(MATRIX_TOOL) suite --manifest "$(CDC_SUITE_MANIFEST)" waves)

.PHONY: test-cdc-optional-wave
test-cdc-optional-wave:
	@if [[ -z "$(WAVE)" ]]; then \
		echo "WAVE is required (example: optional-queues)"; \
		exit 1; \
	fi
	@set -euo pipefail; \
	wave="$(WAVE)"; \
	if [[ " $(CDC_OPTIONAL_WAVES) " != *" $$wave "* ]]; then \
		echo "Unsupported optional WAVE '$$wave'. Use one of: $(CDC_OPTIONAL_WAVES)"; \
		exit 1; \
	fi; \
	target="$(TEST_STATE_OPTIONAL_WAVES_DIR)/$$wave.ok"; \
	if [[ "$(FORCE)" == "1" ]]; then \
		rm -f "$$target"; \
	else \
		if [[ -f "$$target" ]]; then \
			echo "SKIP (cached): $$wave"; \
			exit 0; \
		fi; \
	fi; \
	$(MAKE) test-cdc-optional-wave-run WAVE="$$wave"

.PHONY: test-cdc-optional-wave-run
test-cdc-optional-wave-run:
	@if [[ -z "$(WAVE)" ]]; then \
		echo "WAVE is required (example: optional-queues)"; \
		exit 1; \
	fi
	@set -euo pipefail; \
	wave="$(WAVE)"; \
	item_count=0; \
	while IFS=$$'\t' read -r kind a b c d; do \
		[[ -z "$$kind" ]] && continue; \
		item_count=$$((item_count + 1)); \
		case "$$kind" in \
			SUITE) \
				suite_group="$$a"; \
				suite_path="$$b"; \
				suite_name="$$c"; \
				suite_go_test_args="$$d"; \
				if [[ -z "$$suite_go_test_args" ]]; then \
					suite_go_test_args="$(GO_TEST_ARGS)"; \
				fi; \
				echo "=== optional-wave=$$wave suite=$$suite_group/$$suite_path ==="; \
				$(MAKE) run-tests SUITE_GROUP="$$suite_group" SUITE_PATH="$$suite_path" SUITE_NAME="$$suite_name" GO_TEST_ARGS="$$suite_go_test_args"; \
				;; \
			PKG) \
				pkg_pattern="$$a"; \
				pkg_name="$$b"; \
				pkg_go_test_args="$$c"; \
				echo "=== optional-wave=$$wave package=$$pkg_pattern ==="; \
				$(MAKE) run-go-packages PKG_PATTERN="$$pkg_pattern" PKG_NAME="$$pkg_name" PKG_GO_TEST_ARGS="$$pkg_go_test_args"; \
				;; \
			*) \
				echo "Unknown item kind from optional manifest: $$kind"; \
				exit 1; \
				;; \
		esac; \
	done < <($(MATRIX_TOOL) suite --manifest "$(CDC_OPTIONAL_SUITE_MANIFEST)" emit-wave --wave "$$wave"); \
	if [[ "$$item_count" -eq 0 ]]; then \
		echo "No runnable items found for optional wave '$$wave'"; \
		exit 1; \
	fi; \
	mkdir -p "$(TEST_STATE_OPTIONAL_WAVES_DIR)"; \
	date -u +"%Y-%m-%dT%H:%M:%SZ" > "$(TEST_STATE_OPTIONAL_WAVES_DIR)/$$wave.ok"

$(TEST_STATE_OPTIONAL_WAVES_DIR):
	@mkdir -p "$@"

.SECONDEXPANSION:
$(TEST_STATE_OPTIONAL_WAVES_DIR)/%.ok: $$(COMMON_OPTIONAL_WAVE_DEPS) $$(OPTIONAL_WAVE_DEPS_$$*) | $(TEST_STATE_OPTIONAL_WAVES_DIR)
	@set -euo pipefail; \
	wave="$*"; \
	item_count=0; \
	while IFS=$$'\t' read -r kind a b c d; do \
		[[ -z "$$kind" ]] && continue; \
		item_count=$$((item_count + 1)); \
		case "$$kind" in \
			SUITE) \
				suite_group="$$a"; \
				suite_path="$$b"; \
				suite_name="$$c"; \
				suite_go_test_args="$$d"; \
				if [[ -z "$$suite_go_test_args" ]]; then \
					suite_go_test_args="$(GO_TEST_ARGS)"; \
				fi; \
				echo "=== optional-wave=$$wave suite=$$suite_group/$$suite_path ==="; \
				$(MAKE) run-tests SUITE_GROUP="$$suite_group" SUITE_PATH="$$suite_path" SUITE_NAME="$$suite_name" GO_TEST_ARGS="$$suite_go_test_args"; \
				;; \
			PKG) \
				pkg_pattern="$$a"; \
				pkg_name="$$b"; \
				pkg_go_test_args="$$c"; \
				echo "=== optional-wave=$$wave package=$$pkg_pattern ==="; \
				$(MAKE) run-go-packages PKG_PATTERN="$$pkg_pattern" PKG_NAME="$$pkg_name" PKG_GO_TEST_ARGS="$$pkg_go_test_args"; \
				;; \
			*) \
				echo "Unknown item kind from optional manifest: $$kind"; \
				exit 1; \
				;; \
		esac; \
	done < <($(MATRIX_TOOL) suite --manifest "$(CDC_OPTIONAL_SUITE_MANIFEST)" emit-wave --wave "$$wave"); \
	if [[ "$$item_count" -eq 0 ]]; then \
		echo "No runnable items found for optional wave '$$wave'"; \
		exit 1; \
	fi; \
	date -u +"%Y-%m-%dT%H:%M:%SZ" > "$@"

.PHONY: test-cdc-optional
test-cdc-optional:
	@set -euo pipefail; \
	$(MAKE) test-cdc-optional-verify; \
	while IFS= read -r wave; do \
		[[ -z "$$wave" ]] && continue; \
		echo "=== cdc-optional wave=$$wave ==="; \
		$(MAKE) test-cdc-optional-wave WAVE="$$wave"; \
	done < <($(MATRIX_TOOL) suite --manifest "$(CDC_OPTIONAL_SUITE_MANIFEST)" waves)

.PHONY: test-state-list
test-state-list:
	@set -euo pipefail; \
	state_dir="$(TEST_STATE_WAVES_DIR)"; \
	if [[ ! -d "$$state_dir" ]]; then \
		echo "No test wave state found at $$state_dir"; \
		exit 0; \
	fi; \
	for ok in "$$state_dir"/*.ok; do \
		[[ -e "$$ok" ]] || continue; \
		wave="$$(basename "$$ok" .ok)"; \
		ts="$$(cat "$$ok" 2>/dev/null || true)"; \
		echo "$$wave $$ts"; \
	done | sort

.PHONY: test-state-clear
test-state-clear:
	@if [[ -z "$(WAVE)" ]]; then \
		echo "WAVE is required (example: providers)"; \
		exit 1; \
	fi
	@set -euo pipefail; \
	rm -f "$(TEST_STATE_WAVES_DIR)/$(WAVE).ok"; \
	echo "Cleared wave state: $(WAVE)"

.PHONY: test-state-clear-all
test-state-clear-all:
	@set -euo pipefail; \
	rm -rf "$(TEST_STATE_DIR)"; \
	echo "Cleared all wave state in $(TEST_STATE_DIR)"

.PHONY: test-state-optional-list
test-state-optional-list:
	@set -euo pipefail; \
	state_dir="$(TEST_STATE_OPTIONAL_WAVES_DIR)"; \
	if [[ ! -d "$$state_dir" ]]; then \
		echo "No optional test wave state found at $$state_dir"; \
		exit 0; \
	fi; \
	for ok in "$$state_dir"/*.ok; do \
		[[ -e "$$ok" ]] || continue; \
		wave="$$(basename "$$ok" .ok)"; \
		ts="$$(cat "$$ok" 2>/dev/null || true)"; \
		echo "$$wave $$ts"; \
	done | sort

.PHONY: test-state-optional-clear
test-state-optional-clear:
	@if [[ -z "$(WAVE)" ]]; then \
		echo "WAVE is required (example: optional-queues)"; \
		exit 1; \
	fi
	@set -euo pipefail; \
	rm -f "$(TEST_STATE_OPTIONAL_WAVES_DIR)/$(WAVE).ok"; \
	echo "Cleared optional wave state: $(WAVE)"

.PHONY: test-state-optional-clear-all
test-state-optional-clear-all:
	@set -euo pipefail; \
	rm -rf "$(TEST_STATE_OPTIONAL_WAVES_DIR)"; \
	echo "Cleared all optional wave state in $(TEST_STATE_OPTIONAL_WAVES_DIR)"

.PHONY: test-state-matrix-list
test-state-matrix-list:
	@set -euo pipefail; \
	state_dir="$(TEST_STATE_MATRIX_DIR)"; \
	if [[ ! -d "$$state_dir" ]]; then \
		echo "No matrix state found at $$state_dir"; \
		exit 0; \
	fi; \
	for ok in "$$state_dir"/*.ok; do \
		[[ -e "$$ok" ]] || continue; \
		variant="$$(basename "$$ok" .ok | sed 's|-|/|')"; \
		ts="$$(cat "$$ok" 2>/dev/null || true)"; \
		echo "$$variant $$ts"; \
	done | sort

.PHONY: test-state-matrix-clear
test-state-matrix-clear:
	@if [[ -z "$(SOURCE_VARIANT)" ]]; then \
		echo "SOURCE_VARIANT is required (example: postgres/18)"; \
		exit 1; \
	fi
	@set -euo pipefail; \
	slug="$$(echo "$(SOURCE_VARIANT)" | sed 's|/|-|g')"; \
	rm -f "$(TEST_STATE_MATRIX_DIR)/$$slug.ok"; \
	echo "Cleared matrix state: $(SOURCE_VARIANT)"

.PHONY: test-state-matrix-clear-all
test-state-matrix-clear-all:
	@set -euo pipefail; \
	rm -rf "$(TEST_STATE_MATRIX_DIR)"; \
	echo "Cleared all matrix state in $(TEST_STATE_MATRIX_DIR)"

.PHONY: test-layer
test-layer:
	@set -euo pipefail; \
	layer="$(LAYER)"; \
	db="$(DB)"; \
	case "$$db" in \
		pg2ch) source_db="postgres" ;; \
		mysql2ch) source_db="mysql" ;; \
		mongo2ch) source_db="mongo" ;; \
		kafka2ch) source_db="kafka" ;; \
		*) echo "Unsupported DB alias: $$db. Use one of: $(SUPPORTED_FLOW_DBS) $(SUPPORTED_STREAM_FLOW_DBS)"; exit 1 ;; \
	esac; \
	case "$$layer" in \
		e2e) suite_group="tests/e2e"; suite_path="$$db" ;; \
		evolution|resume|large) suite_group="tests"; suite_path="$$layer/$$db" ;; \
		canon) [[ "$$db" == "kafka2ch" ]] && { echo "canon layer is not defined for $$db"; exit 1; }; suite_group="tests"; suite_path="canon/$$source_db" ;; \
		storage) [[ "$$db" == "kafka2ch" ]] && { echo "storage layer is not defined for $$db"; exit 1; }; suite_group="tests"; suite_path="storage/$$source_db" ;; \
		*) echo "Unsupported layer: $$layer. Use one of: $(SUPPORTED_LAYERS)"; exit 1 ;; \
	esac; \
	if [[ "$$layer" == "resume" ]]; then \
		resume_args="$(GO_TEST_ARGS)"; \
		if [[ "$$resume_args" == "-timeout=15m" ]]; then \
			resume_args="-run \"$(RESUME_TEST_PATTERN)\" -timeout=20m"; \
		fi; \
		$(MAKE) run-tests SUITE_GROUP="$$suite_group" SUITE_PATH="$$suite_path" SUITE_NAME="$$layer-$$db" GO_TEST_ARGS="$$resume_args"; \
	else \
		$(MAKE) run-tests SUITE_GROUP="$$suite_group" SUITE_PATH="$$suite_path" SUITE_NAME="$$layer-$$db"; \
	fi

.PHONY: test-layer-all
test-layer-all:
	@set -euo pipefail; \
	for db in $(SUPPORTED_FLOW_DBS) $(SUPPORTED_STREAM_FLOW_DBS); do \
		echo "=== layer=$(LAYER) db=$$db ==="; \
		$(MAKE) test-layer LAYER="$(LAYER)" DB="$$db"; \
	done

.PHONY: test-layer-optional
test-layer-optional:
	@set -euo pipefail; \
	db="$(DB)"; \
	case "$$db" in \
		kafka2ch|eventhub2ch|kinesis2ch|airbyte2ch|oracle2ch|ch2ch) ;; \
		*) echo "Unsupported optional DB alias: $$db. Use one of: $(SUPPORTED_OPTIONAL_FLOW_DBS)"; exit 1 ;; \
	esac; \
	$(MAKE) run-tests SUITE_GROUP="tests/e2e" SUITE_PATH="$$db" SUITE_NAME="e2e-$$db" GO_TEST_ARGS="$(MATRIX_GO_TEST_ARGS)"

.PHONY: test-db
test-db:
	@set -euo pipefail; \
	for layer in storage canon e2e evolution resume large; do \
		echo "=== layer=$$layer db=$(DB) ==="; \
		$(MAKE) test-layer LAYER="$$layer" DB="$(DB)"; \
	done

.PHONY: test-core
test-core:
	@set -euo pipefail; \
	for db in $(SUPPORTED_FLOW_DBS); do \
		echo "=== core db=$$db ==="; \
		$(MAKE) test-layer LAYER=storage DB="$$db"; \
		$(MAKE) test-layer LAYER=canon DB="$$db"; \
		$(MAKE) test-layer LAYER=e2e DB="$$db"; \
		$(MAKE) test-layer LAYER=resume DB="$$db"; \
	done

.PHONY: test-all-supported
test-all-supported:
	@set -euo pipefail; \
	for db in $(SUPPORTED_FLOW_DBS); do \
		$(MAKE) test-db DB="$$db"; \
	done

.PHONY: test-source-variant
test-source-variant:
	@set -euo pipefail; \
	source_variant="$(SOURCE_VARIANT)"; \
	if [[ -z "$$source_variant" ]]; then \
		echo "SOURCE_VARIANT is required (example: postgres/18)"; \
		exit 1; \
	fi; \
	family="$${source_variant%%/*}"; \
	case "$$family" in \
		postgres) db="pg2ch" ;; \
		mysql) db="mysql2ch" ;; \
		mongo) db="mongo2ch" ;; \
		kafka) db="kafka2ch" ;; \
		*) echo "Unsupported SOURCE_VARIANT family: $$family"; exit 1 ;; \
	esac; \
	echo "=== SOURCE_VARIANT=$$source_variant family=$$family ==="; \
	if [[ "$$family" == "kafka" ]]; then \
		layer_set="$(KAFKA_MATRIX_LAYERS)"; \
		if [[ "$$source_variant" == "kafka/redpanda24" ]]; then \
			layer_set="evolution large"; \
		fi; \
		for layer in $$layer_set; do \
			echo "=== layer=$$layer db=$$db variant=$$source_variant ==="; \
			SOURCE_VARIANT="$$source_variant" GO_TEST_ARGS="$(MATRIX_GO_TEST_ARGS)" $(MAKE) test-layer LAYER="$$layer" DB="$$db"; \
		done; \
	else \
		for layer in $(MATRIX_CORE_LAYERS); do \
			echo "=== layer=$$layer db=$$db variant=$$source_variant ==="; \
			SOURCE_VARIANT="$$source_variant" GO_TEST_ARGS="$(MATRIX_GO_TEST_ARGS)" $(MAKE) test-layer LAYER="$$layer" DB="$$db"; \
		done; \
	fi

.PHONY: test-source-family
test-source-family:
	@set -euo pipefail; \
	case "$(MATRIX_FAMILY)" in \
		postgres) variants="17 18" ;; \
		mysql) variants="mysql84 mariadb118" ;; \
		mongo) variants="6 7" ;; \
		kafka) variants="confluent75 redpanda24" ;; \
		*) echo "Unsupported MATRIX_FAMILY: $(MATRIX_FAMILY)"; exit 1 ;; \
	esac; \
	for v in $$variants; do \
		SOURCE_VARIANT="$(MATRIX_FAMILY)/$$v" $(MAKE) test-source-variant; \
	done

.PHONY: test-source-matrix
test-source-matrix:
	@set -euo pipefail; \
	for source_variant in $(SUPPORTED_SOURCE_VARIANTS); do \
		SOURCE_VARIANT="$$source_variant" $(MAKE) test-source-variant; \
	done

.PHONY: test-matrix-gap-report
test-matrix-gap-report:
	@$(MATRIX_TOOL) gate --matrix "$(MATRIX_CONTRACT)" --wave 1 --write-report "$(MATRIX_REPORT)"
	@$(MATRIX_TOOL) gate --matrix "$(MATRIX_CONTRACT)" --wave 1 --enforce

.PHONY: test-matrix-wave1
test-matrix-wave1:
	@set -euo pipefail; \
	$(MATRIX_TOOL) gate --matrix "$(MATRIX_CONTRACT)" --wave 1 --write-report "$(MATRIX_REPORT)" --enforce; \
	PATH="$$(go env GOPATH)/bin:$$PATH"; \
	command -v gotestsum >/dev/null 2>&1 || { echo "gotestsum is required. Install: go install gotest.tools/gotestsum@latest"; exit 1; }; \
	export RECIPE_CLICKHOUSE_BIN=clickhouse; \
	export USE_TESTCONTAINERS=1; \
	export YA_TEST_RUNNER=1; \
	export YT_PROXY=localhost:8180; \
	export TEST_DEPS_BINARY_PATH=binaries; \
	export LOG_LEVEL=ERROR; \
	export YT_LOG_LEVEL=ERROR; \
	rerun_flag=""; \
	if [[ "$(RERUN_FAILS)" == "1" ]]; then \
		rerun_flag="--rerun-fails=2"; \
	fi; \
	while IFS= read -r dir; do \
		[[ -z "$$dir" ]] && continue; \
		echo "::group::$$dir"; \
		echo "Running matrix wave1 test package: $$dir"; \
		sanitized_dir=$$(echo "$$dir" | sed 's|/|_|g'); \
		gotestsum \
			--junitfile="reports/matrix-wave1_$$sanitized_dir.xml" \
			--junitfile-project-name="matrix-wave1" \
			--junitfile-testsuite-name="short" \
			$$rerun_flag \
			--format $(GOTESTSUM_FORMAT) \
			--packages="./$$dir" \
			-- $(MATRIX_TEST_GO_ARGS); \
		echo "::endgroup::"; \
	done < <($(MATRIX_TOOL) gate --matrix "$(MATRIX_CONTRACT)" --wave 1 --print-required-paths)

.PHONY: test-matrix-wave2
test-matrix-wave2:
	@set -euo pipefail; \
	$(MATRIX_TOOL) gate --matrix "$(MATRIX_CONTRACT)" --wave 2 --write-report "$(MATRIX_REPORT)" --enforce; \
	PATH="$$(go env GOPATH)/bin:$$PATH"; \
	command -v gotestsum >/dev/null 2>&1 || { echo "gotestsum is required. Install: go install gotest.tools/gotestsum@latest"; exit 1; }; \
	export RECIPE_CLICKHOUSE_BIN=clickhouse; \
	export USE_TESTCONTAINERS=1; \
	export YA_TEST_RUNNER=1; \
	export YT_PROXY=localhost:8180; \
	export TEST_DEPS_BINARY_PATH=binaries; \
	export LOG_LEVEL=ERROR; \
	export YT_LOG_LEVEL=ERROR; \
	rerun_flag=""; \
	if [[ "$(RERUN_FAILS)" == "1" ]]; then \
		rerun_flag="--rerun-fails=2"; \
	fi; \
	while IFS= read -r dir; do \
		[[ -z "$$dir" ]] && continue; \
		echo "::group::$$dir"; \
		echo "Running matrix wave2 test package: $$dir"; \
		sanitized_dir=$$(echo "$$dir" | sed 's|/|_|g'); \
		gotestsum \
			--junitfile="reports/matrix-wave2_$$sanitized_dir.xml" \
			--junitfile-project-name="matrix-wave2" \
			--junitfile-testsuite-name="short" \
			$$rerun_flag \
			--format $(GOTESTSUM_FORMAT) \
			--packages="./$$dir" \
			-- $(MATRIX_TEST_GO_ARGS); \
		echo "::endgroup::"; \
	done < <($(MATRIX_TOOL) gate --matrix "$(MATRIX_CONTRACT)" --wave 2 --print-required-paths)

.PHONY: test-matrix-core
test-matrix-core: test-matrix-wave1

# Define variables
HELM_CHART_PATH := ./helm/transfer
IMAGE_NAME := ghcr.io/transferia/transferia-helm
VERSION := $(shell grep '^version:' $(HELM_CHART_PATH)/Chart.yaml | awk '{print $$2}')

# Login to GitHub Container Registry
.PHONY: login-ghcr
login-ghcr:
	echo "${GHCR_TOKEN}" | docker login ghcr.io -u ${GITHUB_USERNAME} --password-stdin

# Package the Helm chart
.PHONY: helm-package
helm-package:
	helm package $(HELM_CHART_PATH) --destination .

# Push the Helm chart as OCI artifact
.PHONY: helm-push
helm-push: helm-package login-ghcr
	helm push ./transfer-$(VERSION).tgz oci://$(IMAGE_NAME)
