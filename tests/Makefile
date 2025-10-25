.PHONY: test test-parallel test-sequential test-local

TEST_ARGS := $(if $(skip-destroy),-skip-destroy=$(skip-destroy)) \
             $(if $(exception),-exception=$(exception)) \
             $(if $(example),-example=$(example)) \
             $(if $(local),-local=$(local))

test:
	go test -v -timeout 60m -run '^TestApplyNoError$$' -args $(TEST_ARGS) .

test-parallel:
	go test -v -timeout 60m -run '^TestApplyAllParallel$$' -args $(TEST_ARGS) .

test-sequential:
	go test -v -timeout 60m -run '^TestApplyAllSequential$$' -args $(TEST_ARGS) .

test-local:
	go test -v -timeout 60m -run '^TestApplyAllLocal$$' -args $(TEST_ARGS) .
