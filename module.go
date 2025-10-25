package validor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

type Module struct {
	Name        string
	Path        string
	Options     *terraform.Options
	Errors      []string
	ApplyFailed bool
}

type ModuleManager struct {
	BaseExamplesPath string
	Config           *Config
}

func NewModuleManager(baseExamplesPath string) *ModuleManager {
	return &ModuleManager{
		BaseExamplesPath: baseExamplesPath,
	}
}

func (mm *ModuleManager) SetConfig(config *Config) {
	mm.Config = config
}

func NewModule(name, path string) *Module {
	return &Module{
		Name: name,
		Path: path,
		Options: &terraform.Options{
			TerraformDir:    path,
			NoColor:         true,
			TerraformBinary: "terraform",
		},
		Errors:      []string{},
		ApplyFailed: false,
	}
}

func (mm *ModuleManager) DiscoverModules() ([]*Module, error) {
	var modules []*Module

	entries, err := os.ReadDir(mm.BaseExamplesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read examples directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			moduleName := entry.Name()
			if mm.Config != nil && slices.Contains(mm.Config.ExceptionList, moduleName) {
				fmt.Printf("Skipping module %s as it is in the exception list\n", moduleName)
				continue
			}
			modulePath := filepath.Join(mm.BaseExamplesPath, moduleName)
			modules = append(modules, NewModule(moduleName, modulePath))
		}
	}

	return modules, nil
}

func (m *Module) Apply(ctx context.Context, t *testing.T) error {
	t.Helper()

	t.Logf("Applying Terraform module: %s", m.Name)
	terraform.WithDefaultRetryableErrors(t, m.Options)

	_, err := terraform.InitAndApplyE(t, m.Options)
	if err != nil {
		m.ApplyFailed = true
		wrappedErr := &ModuleError{ModuleName: m.Name, Operation: "terraform apply", Err: err}
		m.Errors = append(m.Errors, wrappedErr.Error())
		t.Log(redError(wrappedErr.Error()))
		return wrappedErr
	}
	return nil
}

func (m *Module) Destroy(ctx context.Context, t *testing.T) error {
	t.Helper()

	t.Logf("Destroying Terraform module: %s", m.Name)

	_, destroyErr := terraform.DestroyE(t, m.Options)

	if destroyErr != nil && !m.ApplyFailed {
		wrappedErr := &ModuleError{ModuleName: m.Name, Operation: "terraform destroy", Err: destroyErr}
		m.Errors = append(m.Errors, wrappedErr.Error())
		t.Log(redError(wrappedErr.Error()))
	}

	if err := m.Cleanup(ctx, t); err != nil && !m.ApplyFailed {
		wrappedErr := &ModuleError{ModuleName: m.Name, Operation: "cleanup", Err: err}
		m.Errors = append(m.Errors, wrappedErr.Error())
		t.Log(redError(wrappedErr.Error()))
	}

	return destroyErr
}

func (m *Module) Cleanup(ctx context.Context, t *testing.T) error {
	t.Helper()
	t.Logf("Cleaning up in: %s", m.Options.TerraformDir)
	filesToCleanup := []string{"*.terraform*", "*tfstate*", "*.lock.hcl"}

	for _, pattern := range filesToCleanup {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		matches, err := filepath.Glob(filepath.Join(m.Options.TerraformDir, pattern))
		if err != nil {
			return fmt.Errorf("error matching pattern %s: %w", pattern, err)
		}
		for _, filePath := range matches {
			if err := os.RemoveAll(filePath); err != nil {
				return fmt.Errorf("failed to remove %s: %w", filePath, err)
			}
		}
	}
	return nil
}

func PrintModuleSummary(t *testing.T, modules []*Module) {
	t.Helper()

	var failedModules []*Module
	for _, module := range modules {
		if len(module.Errors) > 0 {
			failedModules = append(failedModules, module)
		}
	}

	if len(failedModules) > 0 {
		for _, module := range failedModules {
			t.Log(redError("Module " + module.Name + " failed with errors:"))
			for i, errMsg := range module.Errors {
				errText := fmt.Sprintf("  %d. %s", i+1, errMsg)
				t.Log(redError(errText))
			}
			t.Log("")
		}

		totalText := fmt.Sprintf("TOTAL: %d of %d modules failed", len(failedModules), len(modules))
		t.Log(redError(totalText))
	} else {
		t.Logf("\n==== SUCCESS: All %d modules applied and destroyed successfully ====", len(modules))
	}
}
