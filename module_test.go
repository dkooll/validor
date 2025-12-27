package validor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

func TestNewModule(t *testing.T) {
	tests := []struct {
		name       string
		moduleName string
		path       string
	}{
		{
			name:       "basic module",
			moduleName: "example1",
			path:       "/path/to/example1",
		},
		{
			name:       "module with complex path",
			moduleName: "complex-example",
			path:       "/path/to/examples/complex-example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := NewModule(tt.moduleName, tt.path)

			if module.Name != tt.moduleName {
				t.Errorf("Module.Name = %v, want %v", module.Name, tt.moduleName)
			}
			if module.Path != tt.path {
				t.Errorf("Module.Path = %v, want %v", module.Path, tt.path)
			}
			if module.Options == nil {
				t.Error("Module.Options is nil")
			}
			if module.Options.TerraformDir != tt.path {
				t.Errorf("Module.Options.TerraformDir = %v, want %v", module.Options.TerraformDir, tt.path)
			}
			if !module.Options.NoColor {
				t.Error("Module.Options.NoColor should be true")
			}
			if module.Options.TerraformBinary != "terraform" {
				t.Errorf("Module.Options.TerraformBinary = %v, want terraform", module.Options.TerraformBinary)
			}
			if module.ApplyFailed {
				t.Error("Module.ApplyFailed should be false initially")
			}
			if len(module.Errors) != 0 {
				t.Errorf("Module.Errors should be empty initially, got %v", module.Errors)
			}
		})
	}
}

func TestModuleManager_DiscoverModules(t *testing.T) {
	tmpDir := t.TempDir()

	testModules := []string{"example1", "example2", "example3"}
	for _, mod := range testModules {
		modPath := filepath.Join(tmpDir, mod)
		if err := os.Mkdir(modPath, 0o755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Run("discover all modules", func(t *testing.T) {
		mm := NewModuleManager(tmpDir)
		mm.SetConfig(&Config{})

		modules, err := mm.DiscoverModules()
		if err != nil {
			t.Fatalf("DiscoverModules() error = %v", err)
		}

		if len(modules) != len(testModules) {
			t.Errorf("DiscoverModules() found %d modules, want %d", len(modules), len(testModules))
		}

		moduleNames := make(map[string]bool)
		for _, mod := range modules {
			moduleNames[mod.Name] = true
		}

		for _, expected := range testModules {
			if !moduleNames[expected] {
				t.Errorf("Expected module %s not found", expected)
			}
		}
	})

	t.Run("discover modules with exceptions", func(t *testing.T) {
		mm := NewModuleManager(tmpDir)
		config := &Config{
			Exception:     "example2",
			ExceptionList: []string{"example2"},
		}
		mm.SetConfig(config)

		modules, err := mm.DiscoverModules()
		if err != nil {
			t.Fatalf("DiscoverModules() error = %v", err)
		}

		expectedCount := len(testModules) - 1 // Minus one exception
		if len(modules) != expectedCount {
			t.Errorf("DiscoverModules() with exception found %d modules, want %d", len(modules), expectedCount)
		}

		// Ensure the excepted module is not in the results
		for _, mod := range modules {
			if mod.Name == "example2" {
				t.Error("Excepted module example2 should not be discovered")
			}
		}
	})

	t.Run("discover modules from non-existent directory", func(t *testing.T) {
		mm := NewModuleManager("/non/existent/path")
		mm.SetConfig(&Config{})

		_, err := mm.DiscoverModules()
		if err == nil {
			t.Error("DiscoverModules() should return error for non-existent directory")
		}
	})
}

func TestExtractModuleNames(t *testing.T) {
	tests := []struct {
		name    string
		modules []*Module
		want    []string
	}{
		{
			name:    "empty list",
			modules: []*Module{},
			want:    []string{},
		},
		{
			name: "single module",
			modules: []*Module{
				NewModule("example1", "/path/example1"),
			},
			want: []string{"example1"},
		},
		{
			name: "multiple modules",
			modules: []*Module{
				NewModule("example1", "/path/example1"),
				NewModule("example2", "/path/example2"),
				NewModule("example3", "/path/example3"),
			},
			want: []string{"example1", "example2", "example3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractModuleNames(tt.modules)
			if len(got) != len(tt.want) {
				t.Errorf("extractModuleNames() returned %d names, want %d", len(got), len(tt.want))
				return
			}
			for i, name := range got {
				if name != tt.want[i] {
					t.Errorf("extractModuleNames()[%d] = %v, want %v", i, name, tt.want[i])
				}
			}
		})
	}
}

func TestCreateModulesFromNames(t *testing.T) {
	tests := []struct {
		name        string
		moduleNames []string
		basePath    string
	}{
		{
			name:        "empty list",
			moduleNames: []string{},
			basePath:    "/base/path",
		},
		{
			name:        "single module",
			moduleNames: []string{"example1"},
			basePath:    "/base/path",
		},
		{
			name:        "multiple modules",
			moduleNames: []string{"example1", "example2", "example3"},
			basePath:    "/base/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modules := createModulesFromNames(tt.moduleNames, tt.basePath)

			if len(modules) != len(tt.moduleNames) {
				t.Errorf("createModulesFromNames() created %d modules, want %d", len(modules), len(tt.moduleNames))
				return
			}

			for i, mod := range modules {
				expectedName := tt.moduleNames[i]
				expectedPath := filepath.Join(tt.basePath, expectedName)

				if mod.Name != expectedName {
					t.Errorf("Module[%d].Name = %v, want %v", i, mod.Name, expectedName)
				}
				if mod.Path != expectedPath {
					t.Errorf("Module[%d].Path = %v, want %v", i, mod.Path, expectedPath)
				}
			}
		})
	}
}

func TestPrintModuleSummary(t *testing.T) {
	t.Run("all modules successful", func(t *testing.T) {
		modules := []*Module{
			NewModule("example1", "/path/example1"),
			NewModule("example2", "/path/example2"),
		}

		// This test just ensures PrintModuleSummary doesn't panic with successful modules
		PrintModuleSummary(t, modules)
	})

	t.Run("modules with failures", func(t *testing.T) {
		modules := []*Module{
			NewModule("example1", "/path/example1"),
			{
				Name:   "example2",
				Path:   "/path/example2",
				Errors: []string{"Error 1", "Error 2"},
			},
		}

		// This test just ensures PrintModuleSummary doesn't panic with failed modules
		PrintModuleSummary(t, modules)
	})
}

func TestModule_Cleanup(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create test files
	testFiles := []string{
		".terraform",
		"terraform.tfstate",
		"terraform.tfstate.backup",
		".terraform.lock.hcl",
	}

	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	module := &Module{
		Name: "test",
		Path: tmpDir,
		Options: &terraform.Options{
			TerraformDir: tmpDir,
		},
	}

	// Use a background context
	ctx := testContext(t)

	// Run cleanup
	if err := module.Cleanup(ctx, t); err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}

	// Verify files were removed
	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("File %s should have been removed", file)
		}
	}
}

func TestModule_DestroyErrors(t *testing.T) {
	module := NewModule("test", t.TempDir())

	module.destroyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		return fmt.Errorf("destroy failed")
	}
	module.cleanupHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		return fmt.Errorf("cleanup failed")
	}

	err := module.Destroy(context.Background(), t)
	if err == nil {
		t.Fatalf("expected destroy to return error")
	}

	if len(module.Errors) != 2 {
		t.Fatalf("expected 2 errors recorded, got %d", len(module.Errors))
	}
	if module.Errors[0] == "" || module.Errors[1] == "" {
		t.Fatalf("expected error messages to be populated, got %#v", module.Errors)
	}
}
