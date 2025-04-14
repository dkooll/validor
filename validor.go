package validor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// TestResult represents the result of testing a module
type TestResult struct {
	ModuleName  string
	Success     bool
	ErrorDetail string
	Path        string
}

// TestOptions contains configuration for testing
type TestOptions struct {
	ExamplesPath  string
	Example       string
	SkipDestroy   bool
	Exceptions    string
	UseLocalSrc   bool
	Silent        bool
}

// TestOptionFn is a function that modifies TestOptions
type TestOptionFn func(*TestOptions)

// WithExamplesPath sets the path to examples directory
func WithExamplesPath(path string) TestOptionFn {
	return func(o *TestOptions) {
		o.ExamplesPath = path
	}
}

// WithExample sets a specific example to test
func WithExample(name string) TestOptionFn {
	return func(o *TestOptions) {
		o.Example = name
	}
}

// WithSkipDestroy sets whether to skip destroying resources
func WithSkipDestroy(skip bool) TestOptionFn {
	return func(o *TestOptions) {
		o.SkipDestroy = skip
	}
}

// WithExceptions sets examples to exclude
func WithExceptions(except string) TestOptionFn {
	return func(o *TestOptions) {
		o.Exceptions = except
	}
}

// WithLocalSource sets to use local source references
func WithLocalSource(local bool) TestOptionFn {
	return func(o *TestOptions) {
		o.UseLocalSrc = local
	}
}

// WithSilent suppresses non-essential output
func WithSilent(silent bool) TestOptionFn {
	return func(o *TestOptions) {
		o.Silent = silent
	}
}

// Run executes terraform tests with the given options
func Run(t *testing.T, optFns ...TestOptionFn) ([]TestResult, error) {
	// Set default options
	opts := TestOptions{
		ExamplesPath: "../examples",
		SkipDestroy: false,
	}

	// Apply options
	for _, fn := range optFns {
		fn(&opts)
	}

	// Parse exceptions
	exceptions := make(map[string]bool)
	for _, ex := range strings.Split(opts.Exceptions, ",") {
		ex = strings.TrimSpace(ex)
		if ex != "" {
			exceptions[ex] = true
		}
	}

	// Set up modules to test
	var modules []*Module
	var err error

	// Test specific example or all examples
	if opts.Example != "" {
		if exceptions[opts.Example] {
			if !opts.Silent {
				t.Logf("Skipping %s as it's in the exception list", opts.Example)
			}
			return []TestResult{}, nil
		}

		modulePath := filepath.Join(opts.ExamplesPath, opts.Example)
		modules = []*Module{{
			Name: opts.Example,
			Path: modulePath,
			Options: &terraform.Options{
				TerraformDir: modulePath,
				NoColor:      true,
			},
		}}
	} else {
		modules, err = discoverModules(opts.ExamplesPath, exceptions)
		if err != nil {
			return nil, fmt.Errorf("failed to discover modules: %v", err)
		}
	}

	// Convert to local source if requested
	if opts.UseLocalSrc {
		for _, mod := range modules {
			if err := convertToLocal(mod.Path); err != nil {
				return nil, fmt.Errorf("failed to convert module %s to local source: %v", mod.Name, err)
			}
		}
	}

	// Run tests on modules in parallel
	resultChan := make(chan TestResult, len(modules))

	for _, mod := range modules {
		mod := mod // Create local copy for goroutine

		t.Run(mod.Name, func(t *testing.T) {
			t.Parallel()

			result := TestResult{
				ModuleName: mod.Name,
				Path:       mod.Path,
				Success:    true,
			}

			// Apply the module
			if err := mod.Apply(t); err != nil {
				result.Success = false
				result.ErrorDetail = err.Error()
			}

			// Destroy the module if not skipping
			if !opts.SkipDestroy {
				if destroyErr := mod.Destroy(t); destroyErr != nil {
					if result.Success {
						result.Success = false
						result.ErrorDetail = fmt.Sprintf("destroy failed: %v", destroyErr)
					} else {
						result.ErrorDetail += fmt.Sprintf(" (destroy also failed: %v)", destroyErr)
					}
				}
			}

			resultChan <- result
		})
	}

	// Collect results
	var results []TestResult
	for i := 0; i < len(modules); i++ {
		results = append(results, <-resultChan)
	}

	return results, nil
}

// FormatResult formats a test result
func FormatResult(result TestResult) string {
	if result.Success {
		return fmt.Sprintf("✅ Module %s: Success", result.ModuleName)
	}
	return fmt.Sprintf("❌ Module %s: Failed - %s", result.ModuleName, result.ErrorDetail)
}

// Module represents a Terraform module to test
type Module struct {
	Name    string
	Path    string
	Options *terraform.Options
}

// discoverModules finds all modules in the examples directory
func discoverModules(path string, exceptions map[string]bool) ([]*Module, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read examples directory: %v", err)
	}

	var modules []*Module
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if exceptions[name] {
			continue
		}

		modulePath := filepath.Join(path, name)
		modules = append(modules, &Module{
			Name: name,
			Path: modulePath,
			Options: &terraform.Options{
				TerraformDir: modulePath,
				NoColor:      true,
			},
		})
	}

	return modules, nil
}

// Apply applies the Terraform module
func (m *Module) Apply(t *testing.T) error {
	terraform.WithDefaultRetryableErrors(t, m.Options)
	_, err := terraform.InitAndApplyE(t, m.Options)
	return err
}

// Destroy destroys the Terraform module and cleans up files
func (m *Module) Destroy(t *testing.T) error {
	_, destroyErr := terraform.DestroyE(t, m.Options)

	patterns := []string{"*.terraform*", "*tfstate*", "*.lock.hcl"}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(m.Options.TerraformDir, pattern))
		for _, path := range matches {
			os.RemoveAll(path)
		}
	}

	return destroyErr
}

// ModuleConfig contains information about a module derived from the repo name
type ModuleConfig struct {
	RegistrySource string
	ModuleName     string
	Provider       string
	Namespace      string
}

// detectNamespace tries to determine the registry namespace
func detectNamespace() (string, error) {
	// Try environment variable first
	if ns := os.Getenv("TF_REGISTRY_NAMESPACE"); ns != "" {
		return ns, nil
	}

	// Try git config
	cmd := exec.Command("git", "config", "--get", "user.github")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output)), nil
	}

	// Try organization name from origin remote
	cmd = exec.Command("git", "remote", "get-url", "origin")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		url := strings.TrimSpace(string(output))

		// Extract org name from GitHub URL formats
		if strings.Contains(url, "github.com") {
			// Format: https://github.com/orgname/repo.git or git@github.com:orgname/repo.git
			parts := strings.Split(url, "/")
			if len(parts) > 1 {
				orgPart := parts[len(parts)-2]
				// Handle SSH format
				if strings.Contains(orgPart, ":") {
					orgPart = strings.Split(orgPart, ":")[1]
				}
				return orgPart, nil
			}
		}
	}

	// As a last resort, use parent directory name
	repoCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	repoPath, err := repoCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to detect namespace: %v", err)
	}

	return filepath.Base(strings.TrimSpace(string(repoPath))), nil
}

// getModuleConfig extracts module information from repository name
func getModuleConfig() (*ModuleConfig, error) {
	// Get repository root and name
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git repository root: %v", err)
	}

	repoPath := strings.TrimSpace(string(output))
	repoName := filepath.Base(repoPath)

	// Extract provider and module name from repository name (terraform-provider-module)
	parts := strings.Split(repoName, "-")
	if len(parts) < 3 || parts[0] != "terraform" {
		return nil, fmt.Errorf("repository name does not follow terraform-provider-module format: %s", repoName)
	}

	provider := parts[1]
	moduleName := parts[2]

	// Auto-detect namespace
	namespace, err := detectNamespace()
	if err != nil {
		return nil, err
	}

	registrySource := fmt.Sprintf("%s/%s/%s", namespace, moduleName, provider)

	return &ModuleConfig{
		RegistrySource: registrySource,
		ModuleName:     moduleName,
		Provider:       provider,
		Namespace:      namespace,
	}, nil
}

// convertToLocal converts module sources to local paths
func convertToLocal(modulePath string) error {
	// Get module config with auto-detected namespace
	config, err := getModuleConfig()
	if err != nil {
		return err
	}

	// Find and process terraform files
	var files []string
	filepath.Walk(modulePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".tf") {
			files = append(files, path)
		}
		return nil
	})

	for _, file := range files {
		if err := processFile(file, config); err != nil {
			return fmt.Errorf("failed to process file %s: %v", file, err)
		}
	}

	return nil
}

// processFile processes a terraform file to convert module sources
func processFile(filePath string, config *ModuleConfig) error {
	// Parse the HCL file
	parser := hclparse.NewParser()

	file, diags := parser.ParseHCLFile(filePath)
	if diags.HasErrors() {
		return fmt.Errorf("failed to parse HCL file: %s", diags.Error())
	}

	// Define schema for blocks we're looking for
	content, diags := file.Body.Content(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "module",
				LabelNames: []string{"name"},
			},
		},
	})

	if diags.HasErrors() {
		return fmt.Errorf("failed to read HCL content: %s", diags.Error())
	}

	// Read file content as string
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	lines := strings.Split(string(fileContent), "\n")
	modified := false

	// Iterate through module blocks
	for _, block := range content.Blocks {
		if block.Type != "module" {
			continue
		}

		// Create schema for module attributes
		moduleSchema := &hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{
				{Name: "source", Required: true},
				{Name: "version", Required: false},
			},
		}

		// Get module content
		moduleContent, _ := block.Body.Content(moduleSchema)

		// Get source attribute
		sourceAttr := moduleContent.Attributes["source"]
		if sourceAttr == nil {
			continue
		}

		// Extract source value
		sourceVal, diags := sourceAttr.Expr.Value(nil)
		if diags.HasErrors() || !sourceVal.Type().IsPrimitiveType() {
			continue
		}

		source := sourceVal.AsString()

		// Check if this is our registry source
		if source == config.RegistrySource {
			// Find the source line
			sourceLine := sourceAttr.Range.Start.Line - 1 // HCL line numbers are 1-based

			// Get indentation
			indent := ""
			for _, c := range lines[sourceLine] {
				if c == ' ' || c == '\t' {
					indent += string(c)
				} else {
					break
				}
			}

			// Update the source line to local path
			lines[sourceLine] = fmt.Sprintf("%ssource  = \"../../\"", indent)
			modified = true
		}
	}

	// Write changes back to file if modified
	if modified {
		if err := os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
			return fmt.Errorf("failed to write file: %v", err)
		}
	}

	return nil
}
