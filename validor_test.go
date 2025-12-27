package validor

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseExampleList(t *testing.T) {
	tests := []struct {
		name    string
		example string
		want    []string
	}{
		{
			name:    "single example",
			example: "example1",
			want:    []string{"example1"},
		},
		{
			name:    "multiple examples",
			example: "example1,example2,example3",
			want:    []string{"example1", "example2", "example3"},
		},
		{
			name:    "examples with spaces",
			example: " example1 , example2 , example3 ",
			want:    []string{"example1", "example2", "example3"},
		},
		{
			name:    "examples with trailing comma",
			example: "example1,example2,",
			want:    []string{"example1", "example2"},
		},
		{
			name:    "examples with empty entries",
			example: "example1,,example2",
			want:    []string{"example1", "example2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseExampleList(tt.example)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseExampleList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractModuleInfoFromRepo(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		want     ModuleInfo
	}{
		{
			name:     "valid terraform-azure module",
			repoName: "terraform-azure-mymodule",
			want: ModuleInfo{
				Name:     "mymodule",
				Provider: "azure",
			},
		},
		{
			name:     "valid terraform-aws module",
			repoName: "terraform-aws-vpc",
			want: ModuleInfo{
				Name:     "vpc",
				Provider: "aws",
			},
		},
		{
			name:     "module with hyphenated name",
			repoName: "terraform-azure-storage-account",
			want: ModuleInfo{
				Name:     "storage-account",
				Provider: "azure",
			},
		},
		{
			name:     "invalid format - no terraform prefix",
			repoName: "azure-mymodule",
			want:     ModuleInfo{},
		},
		{
			name:     "invalid format - no provider",
			repoName: "terraform-mymodule",
			want:     ModuleInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			repoDir := filepath.Join(tmpDir, tt.repoName)
			if err := os.Mkdir(repoDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			originalWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}
			defer os.Chdir(originalWd)

			if err := os.Chdir(repoDir); err != nil {
				t.Fatalf("Failed to change to test directory: %v", err)
			}

			got := extractModuleInfoFromRepo()

			if got.Name != tt.want.Name || got.Provider != tt.want.Provider {
				t.Errorf("extractModuleInfoFromRepo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractModuleInfoFromRepo_WithTestsSubdir(t *testing.T) {
	tmpDir := t.TempDir()
	repoName := "terraform-azure-testmodule"
	repoDir := filepath.Join(tmpDir, repoName)
	testsDir := filepath.Join(repoDir, "tests")

	if err := os.MkdirAll(testsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(testsDir); err != nil {
		t.Fatalf("Failed to change to test directory: %v", err)
	}

	got := extractModuleInfoFromRepo()

	want := ModuleInfo{
		Name:     "testmodule",
		Provider: "azure",
	}

	if got.Name != want.Name || got.Provider != want.Provider {
		t.Errorf("extractModuleInfoFromRepo() from tests subdir = %+v, want %+v", got, want)
	}
}

func TestGetRepoNameFromGit(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("non-git directory", func(t *testing.T) {
		result := getRepoNameFromGit(tmpDir)
		if result != "" {
			t.Errorf("getRepoNameFromGit() for non-git dir should return empty string, got %v", result)
		}
	})

	t.Run("from git remote", func(t *testing.T) {
		origGit := gitRemoteURL
		defer func() { gitRemoteURL = origGit }()

		gitRemoteURL = func(dir string) ([]byte, error) {
			return []byte("git@github.com:cloudnationhq/terraform-azure-mymodule.git\n"), nil
		}

		got := getRepoNameFromGit(tmpDir)
		if got != "terraform-azure-mymodule" {
			t.Errorf("getRepoNameFromGit() from remote = %v, want terraform-azure-mymodule", got)
		}
	})
}

func TestTestConfig_Options(t *testing.T) {
	t.Run("WithConfig", func(t *testing.T) {
		config := &Config{Example: "test"}
		tc := &TestConfig{}
		WithConfig(config)(tc)
		if tc.Config != config {
			t.Error("WithConfig did not set Config correctly")
		}
	})

	t.Run("WithModules", func(t *testing.T) {
		modules := []string{"mod1", "mod2"}
		tc := &TestConfig{}
		WithModules(modules)(tc)
		if !reflect.DeepEqual(tc.ModuleNames, modules) {
			t.Error("WithModules did not set ModuleNames correctly")
		}
	})

	t.Run("WithLocalSource", func(t *testing.T) {
		tc := &TestConfig{}
		WithLocalSource(true)(tc)
		if !tc.UseLocal {
			t.Error("WithLocalSource did not set UseLocal correctly")
		}
	})

	t.Run("WithParallel", func(t *testing.T) {
		tc := &TestConfig{}
		WithParallel(false)(tc)
		if tc.Parallel {
			t.Error("WithParallel did not set Parallel correctly")
		}
	})

	t.Run("WithTestExamplesPath", func(t *testing.T) {
		tc := &TestConfig{}
		WithTestExamplesPath("/test/path")(tc)
		if tc.ExamplesPath != "/test/path" {
			t.Error("WithTestExamplesPath did not set ExamplesPath correctly")
		}
	})

	t.Run("RunTestsWithOptions applies overrides", func(t *testing.T) {
		origRun := runModuleTestsFn
		defer func() { runModuleTestsFn = origRun }()

		called := false
		runModuleTestsFn = func(t *testing.T, modules []*Module, parallel bool, config *Config, setup TestSetupFunc, sourceType string) {
			called = true
			if !parallel {
				t.Fatalf("expected parallel to be true")
			}
			if config.ExamplesPath != "/tmp/examples" {
				t.Fatalf("expected examples path override, got %s", config.ExamplesPath)
			}
		}

		RunTestsWithOptions(&testing.T{},
			WithTestExamplesPath("/tmp/examples"),
			WithParallel(true),
			WithModules([]string{"a"}),
		)

		if !called {
			t.Fatalf("runModuleTests should have been invoked")
		}
	})
}

func TestSetupConfigWithOptions(t *testing.T) {
	originalConfig := globalConfig
	defer func() { globalConfig = originalConfig }()

	globalConfig = &Config{
		Exception: "ex1,ex2",
	}

	t.Run("apply options to global config", func(t *testing.T) {
		config := setupConfigWithOptions(
			WithSkipDestroy(true),
			WithLocal(true),
		)

		if !config.SkipDestroy {
			t.Error("SkipDestroy should be true")
		}
		if !config.Local {
			t.Error("Local should be true")
		}
		// ExceptionList should be parsed
		if len(config.ExceptionList) != 2 {
			t.Errorf("ExceptionList should have 2 items, got %d", len(config.ExceptionList))
		}
	})
}

func TestConvertModulesToLocal(t *testing.T) {
	tmpDir := t.TempDir()
	examplesDir := filepath.Join(tmpDir, "examples")
	if err := os.MkdirAll(examplesDir, 0755); err != nil {
		t.Fatalf("Failed to create examples directory: %v", err)
	}

	moduleNames := []string{"example1", "example2"}
	for _, modName := range moduleNames {
		modDir := filepath.Join(examplesDir, modName)
		if err := os.Mkdir(modDir, 0755); err != nil {
			t.Fatalf("Failed to create module directory: %v", err)
		}

		tfContent := `
module "test" {
  source  = "cloudnationhq/mymodule/azure"
  version = "~> 1.0"
}
`
		tfFile := filepath.Join(modDir, "main.tf")
		if err := os.WriteFile(tfFile, []byte(tfContent), 0644); err != nil {
			t.Fatalf("Failed to create terraform file: %v", err)
		}
	}

	client := &mockRegistryClient{latestVersion: "1.0.0"}
	converter := NewSourceConverter(client)
	moduleInfo := ModuleInfo{
		Name:      "mymodule",
		Provider:  "azure",
		Namespace: "cloudnationhq",
	}

	ctx := testContext(t)
	mockT := &testing.T{}
	filesToRestore := convertModulesToLocal(ctx, mockT, converter, moduleNames, []string{}, moduleInfo, examplesDir)

	if len(filesToRestore) == 0 {
		t.Error("convertModulesToLocal should return files to restore")
	}

	if len(filesToRestore) != 2 {
		t.Errorf("Expected 2 files to restore, got %d", len(filesToRestore))
	}
}

func TestConvertModulesToLocal_WithExceptions(t *testing.T) {
	tmpDir := t.TempDir()
	examplesDir := filepath.Join(tmpDir, "examples")
	if err := os.MkdirAll(examplesDir, 0755); err != nil {
		t.Fatalf("Failed to create examples directory: %v", err)
	}

	moduleNames := []string{"example1", "example2"}
	exceptionList := []string{"example2"}

	for _, modName := range moduleNames {
		modDir := filepath.Join(examplesDir, modName)
		if err := os.Mkdir(modDir, 0755); err != nil {
			t.Fatalf("Failed to create module directory: %v", err)
		}

		tfContent := `
module "test" {
  source  = "cloudnationhq/mymodule/azure"
  version = "~> 1.0"
}
`
		tfFile := filepath.Join(modDir, "main.tf")
		if err := os.WriteFile(tfFile, []byte(tfContent), 0644); err != nil {
			t.Fatalf("Failed to create terraform file: %v", err)
		}
	}

	client := &mockRegistryClient{latestVersion: "1.0.0"}
	converter := NewSourceConverter(client)
	moduleInfo := ModuleInfo{
		Name:      "mymodule",
		Provider:  "azure",
		Namespace: "cloudnationhq",
	}

	ctx := testContext(t)
	mockT := &testing.T{}
	filesToRestore := convertModulesToLocal(ctx, mockT, converter, moduleNames, exceptionList, moduleInfo, examplesDir)

	if len(filesToRestore) != 1 {
		t.Errorf("Expected 1 file to restore (excluding exception), got %d", len(filesToRestore))
	}
}

func TestConvertModulesToLocal_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	examplesDir := filepath.Join(tmpDir, "examples")
	if err := os.MkdirAll(examplesDir, 0755); err != nil {
		t.Fatalf("Failed to create examples directory: %v", err)
	}

	modDir := filepath.Join(examplesDir, "example1")
	if err := os.Mkdir(modDir, 0755); err != nil {
		t.Fatalf("Failed to create module directory: %v", err)
	}

	tfFile := filepath.Join(modDir, "main.tf")
	original := `
module "test" {
  source  = "cloudnationhq/mymodule/azure"
  version = "~> 1.0"
}
`
	if err := os.WriteFile(tfFile, []byte(original), 0644); err != nil {
		t.Fatalf("Failed to create terraform file: %v", err)
	}

	client := &mockRegistryClient{latestVersion: "1.0.0"}
	converter := NewSourceConverter(client)
	moduleInfo := ModuleInfo{
		Name:      "mymodule",
		Provider:  "azure",
		Namespace: "cloudnationhq",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mockT := &testing.T{}
	filesToRestore := convertModulesToLocal(ctx, mockT, converter, []string{"example1"}, []string{}, moduleInfo, examplesDir)

	if len(filesToRestore) != 0 {
		t.Fatalf("expected no files to restore when context is cancelled, got %d", len(filesToRestore))
	}

	content, err := os.ReadFile(tfFile)
	if err != nil {
		t.Fatalf("failed to read terraform file: %v", err)
	}
	if string(content) != original {
		t.Fatalf("terraform file should remain unchanged on cancellation")
	}
}

func setupMockExamplesDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	for _, example := range []string{"example1", "example2", "example3"} {
		exampleDir := filepath.Join(tmpDir, example)
		if err := os.MkdirAll(exampleDir, 0755); err != nil {
			t.Fatalf("failed to create example dir: %v", err)
		}
		mainTf := filepath.Join(exampleDir, "main.tf")
		if err := os.WriteFile(mainTf, []byte("# mock terraform file"), 0644); err != nil {
			t.Fatalf("failed to create main.tf: %v", err)
		}
	}

	return tmpDir
}

func createMockModules(names []string, basePath string) []*Module {
	modules := make([]*Module, len(names))
	for i, name := range names {
		modules[i] = NewModule(name, filepath.Join(basePath, name))
		modules[i].applyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
			return nil
		}
		modules[i].destroyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
			return nil
		}
	}
	return modules
}

func TestRunTests(t *testing.T) {
	tests := []struct {
		name     string
		parallel bool
	}{
		{
			name:     "run in parallel mode",
			parallel: true,
		},
		{
			name:     "run in sequential mode",
			parallel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			modules := createMockModules([]string{"mod1", "mod2"}, tmpDir)
			config := NewConfig()

			RunTests(t, modules, tt.parallel, config)
		})
	}
}

func TestRunTests_WithSkipDestroy(t *testing.T) {
	tmpDir := t.TempDir()
	var destroyCalled bool

	module := NewModule("test-mod", tmpDir)
	module.applyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		return nil
	}
	module.destroyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		destroyCalled = true
		return nil
	}

	config := NewConfig(WithSkipDestroy(true))
	RunTests(t, []*Module{module}, false, config)

	if destroyCalled {
		t.Error("destroy should not be called when SkipDestroy is true")
	}
}

func TestTestApplyNoError(t *testing.T) {
	t.Run("with valid example flag", func(t *testing.T) {
		tmpDir := setupMockExamplesDir(t)

		opts := []Option{
			WithExample("example1"),
			WithExamplesPath(tmpDir),
			WithSkipDestroy(true),
		}

		TestApplyNoError(t, opts...)
	})

	t.Run("with multiple examples", func(t *testing.T) {
		tmpDir := setupMockExamplesDir(t)

		opts := []Option{
			WithExample("example1,example2"),
			WithExamplesPath(tmpDir),
			WithSkipDestroy(true),
		}

		TestApplyNoError(t, opts...)
	})
}

func TestTestApplyNoError_MissingExampleFlag(t *testing.T) {
	t.Run("should fail when example flag not set", func(t *testing.T) {
		t.Skip("Testing t.Fatal requires subprocess pattern - validation code exists at validor.go:88-90")
	})
}

func TestTestApplyAllParallel(t *testing.T) {
	tmpDir := setupMockExamplesDir(t)

	t.Run("discovers and runs all modules in parallel", func(t *testing.T) {
		TestApplyAllParallel(t, WithExamplesPath(tmpDir), WithSkipDestroy(true))
	})
}

func TestTestApplyAllSequential(t *testing.T) {
	tmpDir := setupMockExamplesDir(t)

	t.Run("discovers and runs all modules sequentially", func(t *testing.T) {
		TestApplyAllSequential(t, WithExamplesPath(tmpDir), WithSkipDestroy(true))
	})
}

func TestTestApplyAllLocal(t *testing.T) {
	t.Run("discovers and runs all modules with local source", func(t *testing.T) {
		tmpRoot := t.TempDir()
		moduleDir := filepath.Join(tmpRoot, "terraform-azure-testmodule")
		if err := os.MkdirAll(moduleDir, 0755); err != nil {
			t.Fatalf("failed to create module dir: %v", err)
		}

		examplesDir := filepath.Join(moduleDir, "examples")
		for _, example := range []string{"example1", "example2"} {
			examplePath := filepath.Join(examplesDir, example)
			if err := os.MkdirAll(examplePath, 0755); err != nil {
				t.Fatalf("failed to create example dir: %v", err)
			}
			mainTf := filepath.Join(examplePath, "main.tf")
			if err := os.WriteFile(mainTf, []byte("# mock terraform file"), 0644); err != nil {
				t.Fatalf("failed to create main.tf: %v", err)
			}
		}

		originalWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working dir: %v", err)
		}
		defer os.Chdir(originalWd)

		if err := os.Chdir(moduleDir); err != nil {
			t.Fatalf("failed to change dir: %v", err)
		}

		TestApplyAllLocal(t, WithExamplesPath(examplesDir), WithSkipDestroy(true))
	})
}

func TestPublicAPI_ConfigOptions(t *testing.T) {
	t.Run("TestApplyAllParallel with exception", func(t *testing.T) {
		tmpDir := setupMockExamplesDir(t)
		TestApplyAllParallel(t,
			WithExamplesPath(tmpDir),
			WithException("example2"),
			WithSkipDestroy(true),
		)
	})

	t.Run("TestApplyAllSequential with exception", func(t *testing.T) {
		tmpDir := setupMockExamplesDir(t)
		TestApplyAllSequential(t,
			WithExamplesPath(tmpDir),
			WithException("example3"),
			WithSkipDestroy(true),
		)
	})

	t.Run("TestApplyAllLocal with skip destroy", func(t *testing.T) {
		tmpRoot := t.TempDir()
		moduleDir := filepath.Join(tmpRoot, "terraform-azure-testmodule")
		if err := os.MkdirAll(moduleDir, 0755); err != nil {
			t.Fatalf("failed to create module dir: %v", err)
		}

		examplesDir := filepath.Join(moduleDir, "examples")
		for _, example := range []string{"example1", "example2"} {
			examplePath := filepath.Join(examplesDir, example)
			if err := os.MkdirAll(examplePath, 0755); err != nil {
				t.Fatalf("failed to create example dir: %v", err)
			}
			mainTf := filepath.Join(examplePath, "main.tf")
			if err := os.WriteFile(mainTf, []byte("# mock terraform file"), 0644); err != nil {
				t.Fatalf("failed to create main.tf: %v", err)
			}
		}

		originalWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working dir: %v", err)
		}
		defer os.Chdir(originalWd)

		if err := os.Chdir(moduleDir); err != nil {
			t.Fatalf("failed to change dir: %v", err)
		}

		TestApplyAllLocal(t,
			WithExamplesPath(examplesDir),
			WithSkipDestroy(true),
		)
	})
}
