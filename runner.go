package validor

import (
	"sync"
	"testing"
)

// RunTests executes tests for multiple modules
func RunTests(t *testing.T, modules []*Module, parallel bool) {
	// Use a mutex to protect access to the global error collector
	var mutex sync.Mutex
	var failedModules []*Module

	for _, module := range modules {
		module := module // Create a new variable for each iteration
		t.Run(module.Name, func(t *testing.T) {
			if parallel {
				t.Parallel()
			}

			// Defer Destroy to ensure cleanup happens, regardless of Apply success or failure
			if !skipDestroy {
				defer func() {
					if err := module.Destroy(t); err != nil && !module.ApplyFailed {
						t.Logf("Warning: Cleanup for module %s failed: %v", module.Name, err)
					}
				}()
			}

			// Apply the module and collect errors
			if err := module.Apply(t); err != nil {
				// Mark this test as failed
				t.Fail()

				// Thread-safe addition to failedModules
				mutex.Lock()
				failedModules = append(failedModules, module)
				mutex.Unlock()
			} else {
				t.Logf("✓ Module %s applied successfully", module.Name)
			}
		})
	}

	// After all tests are complete, log the summary of errors if any
	t.Cleanup(func() {
		PrintModuleSummary(t, modules)
	})
}
