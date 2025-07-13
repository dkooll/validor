# validor [![Go Reference](https://pkg.go.dev/badge/github.com/dkooll/validor.svg)](https://pkg.go.dev/github.com/dkooll/validor)

Validor streamlines azure terraform module testing by automatically applying and destroying resources, ensuring efficient and reliable validation.

## Installation

```zsh
go get github.com/dkooll/validor
```

## Usage

as a local test with command line flags:

```go
package tests

import (
	"testing"

	"github.com/dkooll/validor"
)

func TestApplyNoError(t *testing.T) {
	validor.TestApplyNoError(t)
}

func TestApplyAllParallel(t *testing.T) {
	validor.TestApplyAllParallel(t)
}
```

with a makefile:

```make
.PHONY: test test-parallel

TEST_ARGS := $(if $(skip-destroy),-skip-destroy=$(skip-destroy)) \
             $(if $(exception),-exception=$(exception)) \
             $(if $(example),-example=$(example))

test:
	cd tests && go test -v -timeout 60m -run '^TestApplyNoError$$' -args $(TEST_ARGS) .

test-parallel:
	cd tests && go test -v -timeout 60m -run '^TestApplyAllParallel$$' -args $(TEST_ARGS) .
```

## Features

Automated testing of terraform modules through apply and destroy operations.

Parallel execution support for faster module testing.

Exception handling to exclude specific modules from testing.

Comprehensive error reporting and test result summaries.

Automatic cleanup of Terraform state files and other artifacts.

Support for skipping destroy operations when needed for debugging.

## Options

Validor supports a functional options pattern for configuration:

`-skip-destroy`: Skip running terraform destroy after apply. Default is false.

`-exception:`: Comma separated list of examples to exclude

`-example`: Specific example to test (required for single module tests)

## Contributors

We welcome contributions from the community! Whether it's reporting a bug, suggesting a new feature, or submitting a pull request, your input is highly valued. <br><br>

<a href="https://github.com/dkooll/validor/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=dkooll/validor" />
</a>

## Notes

The package assumes your examples are in an `../examples` directory relative to your tests directory.

Command-line flags take highest priority when specified.

This approach supports both local testing and CI/CD environments with the same code.

Terraform must be installed and available in the PATH.
