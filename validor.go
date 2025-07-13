// Package validor provides tools for testing Terraform modules
package validor

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
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
	flag.StringVar(&example, "example", "", "Specific example(s) to test (comma-separated)")
}

// TestApplyNoError tests specific Terraform modules listed in the -example flag
func TestApplyNoError(t *testing.T) {
	flag.Parse()
	parseExceptionList()

	if example == "" {
		t.Fatal(redError("-example flag is not set"))
	}

	// Parse the example flag as a comma-separated list
	exampleList := strings.Split(example, ",")

	var allModules []*Module
	// Use a mutex to protect access to the allModules slice
	var mutex sync.Mutex

	// Test each specified example
	for _, ex := range exampleList {
		ex = strings.TrimSpace(ex)
		if ex == "" {
			continue
		}

		if exceptionList[ex] {
			t.Logf("Skipping example %s as it is in the exception list", ex)
			continue
		}

		// Run the example as a subtest to get better reporting
		t.Run(ex, func(t *testing.T) {
			t.Parallel()
			modulePath := filepath.Join("..", "examples", ex)
			module := NewModule(ex, modulePath)

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

			// Add this module to the slice in a thread-safe way
			mutex.Lock()
			allModules = append(allModules, module)
			mutex.Unlock()
		})
	}

	// Wait for all subtests to complete and then print a final summary
	t.Cleanup(func() {
		PrintModuleSummary(t, allModules)
	})
}

// TestApplyAllParallel discovers and tests all Terraform modules in parallel
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
