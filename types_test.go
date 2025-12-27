package validor

import (
	"errors"
	"testing"
)

func TestNewTestResults(t *testing.T) {
	results := NewTestResults()

	if results == nil {
		t.Error("NewTestResults() should not return nil")
	}

	modules, failedModules := results.GetResults()
	if len(modules) != 0 {
		t.Errorf("New TestResults should have 0 modules, got %d", len(modules))
	}
	if len(failedModules) != 0 {
		t.Errorf("New TestResults should have 0 failed modules, got %d", len(failedModules))
	}
}

func TestTestResults_AddModule(t *testing.T) {
	results := NewTestResults()

	t.Run("add successful module", func(t *testing.T) {
		module := NewModule("test1", "/path/test1")
		results.AddModule(module)

		modules, failedModules := results.GetResults()
		if len(modules) != 1 {
			t.Errorf("Expected 1 module, got %d", len(modules))
		}
		if len(failedModules) != 0 {
			t.Errorf("Expected 0 failed modules, got %d", len(failedModules))
		}
	})

	t.Run("add failed module", func(t *testing.T) {
		module := NewModule("test2", "/path/test2")
		module.Errors = append(module.Errors, "test error")
		results.AddModule(module)

		modules, failedModules := results.GetResults()
		if len(modules) != 2 {
			t.Errorf("Expected 2 modules total, got %d", len(modules))
		}
		if len(failedModules) != 1 {
			t.Errorf("Expected 1 failed module, got %d", len(failedModules))
		}
	})

	t.Run("concurrent add operations", func(t *testing.T) {
		results := NewTestResults()
		done := make(chan bool)

		// Add modules concurrently
		for i := range 10 {
			go func(id int) {
				module := NewModule("test", "/path")
				if id%2 == 0 {
					module.Errors = append(module.Errors, "error")
				}
				results.AddModule(module)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for range 10 {
			<-done
		}

		modules, failedModules := results.GetResults()
		if len(modules) != 10 {
			t.Errorf("Expected 10 modules, got %d", len(modules))
		}
		if len(failedModules) != 5 {
			t.Errorf("Expected 5 failed modules, got %d", len(failedModules))
		}
	})
}

func TestModuleError_Error(t *testing.T) {
	tests := []struct {
		name       string
		moduleName string
		operation  string
		err        error
		wantSubstr string
	}{
		{
			name:       "apply error",
			moduleName: "test-module",
			operation:  "terraform apply",
			err:        errors.New("resource not found"),
			wantSubstr: "terraform apply failed for module test-module: resource not found",
		},
		{
			name:       "destroy error",
			moduleName: "example-module",
			operation:  "terraform destroy",
			err:        errors.New("timeout"),
			wantSubstr: "terraform destroy failed for module example-module: timeout",
		},
		{
			name:       "cleanup error",
			moduleName: "my-module",
			operation:  "cleanup",
			err:        errors.New("permission denied"),
			wantSubstr: "cleanup failed for module my-module: permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ModuleError{
				ModuleName: tt.moduleName,
				Operation:  tt.operation,
				Err:        tt.err,
			}

			got := err.Error()
			if got != tt.wantSubstr {
				t.Errorf("ModuleError.Error() = %v, want %v", got, tt.wantSubstr)
			}
		})
	}
}

func TestModuleError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	moduleErr := &ModuleError{
		ModuleName: "test",
		Operation:  "apply",
		Err:        originalErr,
	}

	unwrapped := moduleErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() should return original error")
	}

	// Test with errors.Is
	if !errors.Is(moduleErr, originalErr) {
		t.Error("errors.Is should work with ModuleError")
	}
}

func TestModuleInfo(t *testing.T) {
	info := ModuleInfo{
		Name:      "mymodule",
		Provider:  "azure",
		Namespace: "cloudnationhq",
	}

	if info.Name != "mymodule" {
		t.Errorf("ModuleInfo.Name = %v, want mymodule", info.Name)
	}
	if info.Provider != "azure" {
		t.Errorf("ModuleInfo.Provider = %v, want azure", info.Provider)
	}
	if info.Namespace != "cloudnationhq" {
		t.Errorf("ModuleInfo.Namespace = %v, want cloudnationhq", info.Namespace)
	}
}

func TestFileRestore(t *testing.T) {
	restore := FileRestore{
		Path:            "/path/to/file.tf",
		OriginalContent: "original content",
		ModuleName:      "test-module",
		Provider:        "azure",
		Namespace:       "cloudnationhq",
	}

	if restore.Path != "/path/to/file.tf" {
		t.Errorf("FileRestore.Path = %v, want /path/to/file.tf", restore.Path)
	}
	if restore.OriginalContent != "original content" {
		t.Errorf("FileRestore.OriginalContent = %v, want 'original content'", restore.OriginalContent)
	}
	if restore.ModuleName != "test-module" {
		t.Errorf("FileRestore.ModuleName = %v, want test-module", restore.ModuleName)
	}
	if restore.Provider != "azure" {
		t.Errorf("FileRestore.Provider = %v, want azure", restore.Provider)
	}
	if restore.Namespace != "cloudnationhq" {
		t.Errorf("FileRestore.Namespace = %v, want cloudnationhq", restore.Namespace)
	}
}

func TestTerraformRegistryResponse(t *testing.T) {
	resp := TerraformRegistryResponse{
		Versions: []struct {
			Version string `json:"version"`
		}{
			{Version: "1.0.0"},
			{Version: "1.1.0"},
			{Version: "2.0.0"},
		},
	}

	if len(resp.Versions) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(resp.Versions))
	}
	if resp.Versions[0].Version != "1.0.0" {
		t.Errorf("First version = %v, want 1.0.0", resp.Versions[0].Version)
	}
	if resp.Versions[2].Version != "2.0.0" {
		t.Errorf("Last version = %v, want 2.0.0", resp.Versions[2].Version)
	}
}
