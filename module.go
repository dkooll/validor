package validor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

// Module represents a Terraform module with test configuration and results
type Module struct {
	Name        string
	Path        string
	Options     *terraform.Options
	Errors      []string
	ApplyFailed bool
}

// ModuleManager discovers Terraform modules in a directory structure
type ModuleManager struct {
	BaseExamplesPath string
}

// NewModuleManager creates a ModuleManager with the specified examples directory
func NewModuleManager(baseExamplesPath string) *ModuleManager {
	return &ModuleManager{
		BaseExamplesPath: baseExamplesPath,
	}
}

// NewModule creates a Module with the specified name and path
func NewModule(name, path string) *Module {
	return &Module{
		Name: name,
		Path: path,
		Options: &terraform.Options{
			TerraformDir: path,
			NoColor:      true,
		},
		Errors:      []string{},
		ApplyFailed: false,
	}
}

// DiscoverModules scans the examples directory and returns all discoverable modules
func (mm *ModuleManager) DiscoverModules() ([]*Module, error) {
	var modules []*Module

	entries, err := os.ReadDir(mm.BaseExamplesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read examples directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			moduleName := entry.Name()
			if exceptionList[moduleName] {
				fmt.Printf("Skipping module %s as it is in the exception list\n", moduleName)
				continue
			}
			modulePath := filepath.Join(mm.BaseExamplesPath, moduleName)
			modules = append(modules, NewModule(moduleName, modulePath))
		}
	}

	return modules, nil
}

// Apply initializes and applies the Terraform module
func (m *Module) Apply(t *testing.T) error {
	t.Helper()
	t.Logf("Applying Terraform module: %s", m.Name)
	terraform.WithDefaultRetryableErrors(t, m.Options)
	_, err := terraform.InitAndApplyE(t, m.Options)
	if err != nil {
		m.ApplyFailed = true
		errMsg := fmt.Sprintf("Apply failed: %v", err)
		m.Errors = append(m.Errors, errMsg)
		t.Log(redError(errMsg))
	}
	return err
}

// Destroy tears down the Terraform module and cleans up generated files
func (m *Module) Destroy(t *testing.T) error {
	t.Helper()
	t.Logf("Destroying Terraform module: %s", m.Name)
	_, destroyErr := terraform.DestroyE(t, m.Options)

	// If we had a failure in Apply, we expect an error in Destroy too
	// Only add the destroy error to the error list if Apply was successful
	if destroyErr != nil && !m.ApplyFailed {
		errMsg := fmt.Sprintf("Destroy failed: %v", destroyErr)
		m.Errors = append(m.Errors, errMsg)
		t.Log(redError(errMsg))
	}

	// Clean up files regardless of apply/destroy status
	if err := m.cleanupFiles(t); err != nil && !m.ApplyFailed {
		errMsg := fmt.Sprintf("Cleanup failed: %v", err)
		m.Errors = append(m.Errors, errMsg)
		t.Log(redError(errMsg))
	}

	// Return the actual error for proper test flow control
	return destroyErr
}

// cleanupFiles removes Terraform-generated files after testing
func (m *Module) cleanupFiles(t *testing.T) error {
	t.Helper()
	t.Logf("Cleaning up in: %s", m.Options.TerraformDir)
	filesToCleanup := []string{"*.terraform*", "*tfstate*", "*.lock.hcl"}

	for _, pattern := range filesToCleanup {
		matches, err := filepath.Glob(filepath.Join(m.Options.TerraformDir, pattern))
		if err != nil {
			return fmt.Errorf("error matching pattern %s: %v", pattern, err)
		}
		for _, filePath := range matches {
			if err := os.RemoveAll(filePath); err != nil {
				return fmt.Errorf("failed to remove %s: %v", filePath, err)
			}
		}
	}
	return nil
}

// PrintModuleSummary outputs a formatted summary of module test results
func PrintModuleSummary(t *testing.T, modules []*Module) {
	t.Helper()

	var failedModules []*Module
	for _, module := range modules {
		if len(module.Errors) > 0 {
			failedModules = append(failedModules, module)
		}
	}

	if len(failedModules) > 0 {
		// Print details for each failed module
		for _, module := range failedModules {
			t.Log(redError("Module " + module.Name + " failed with errors:"))
			for i, errMsg := range module.Errors {
				errText := fmt.Sprintf("  %d. %s", i+1, errMsg)
				t.Log(redError(errText))
			}
			t.Log("") // Empty line for better readability
		}

		// Print a count summary at the end
		totalText := fmt.Sprintf("TOTAL: %d of %d modules failed", len(failedModules), len(modules))
		t.Log(redError(totalText))
	} else {
		t.Logf("\n==== SUCCESS: All %d modules applied and destroyed successfully ====", len(modules))
	}
}
