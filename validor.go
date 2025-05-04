package validor

import (
	"flag"
	"fmt"
	"path/filepath"
	"testing"
)

var (
	skipDestroy   bool
	exception     string
	example       string
	exceptionList map[string]bool
)

func init() {
	flag.BoolVar(&skipDestroy, "skip-destroy", false, "Skip running terraform destroy after apply")
	flag.StringVar(&exception, "exception", "", "Comma-separated list of examples to exclude")
	flag.StringVar(&example, "example", "", "Specific example to test")
}

// TestApplyNoError tests a single Terraform module
func TestApplyNoError(t *testing.T) {
	flag.Parse()
	parseExceptionList()

	if example == "" {
		t.Fatal(redError("-example flag is not set"))
	}

	if exceptionList[example] {
		t.Skipf("Skipping example %s as it is in the exception list", example)
	}

	modulePath := filepath.Join("..", "examples", example)
	module := NewModule(example, modulePath)

	if err := module.Apply(t); err != nil {
		t.Fail()
	} else {
		t.Logf("✓ Module %s applied successfully", module.Name)
	}

	if !skipDestroy {
		if err := module.Destroy(t); err != nil && !module.ApplyFailed {
			t.Logf("Cleanup failed for module %s: %v", module.Name, err)
		}
	}

	// Print a summary for single module test
	PrintModuleSummary(t, []*Module{module})
}

// TestApplyAllParallel tests all Terraform modules in parallel
func TestApplyAllParallel(t *testing.T) {
	flag.Parse()
	parseExceptionList()

	manager := NewModuleManager(filepath.Join("..", "examples"))
	modules, err := manager.DiscoverModules()
	if err != nil {
		errText := fmt.Sprintf("Failed to discover modules: %v", err)
		t.Fatal(redError(errText))
	}

	RunTests(t, modules, true)
}