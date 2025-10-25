// Package validor provides testing utilities for Terraform modules.
package validor

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var globalConfig *Config

type Config struct {
	SkipDestroy   bool
	Exception     string
	Example       string
	Local         bool
	ExceptionList []string
	Namespace     string
}

type Option func(*Config)

func WithSkipDestroy(skip bool) Option {
	return func(c *Config) { c.SkipDestroy = skip }
}

func WithException(exception string) Option {
	return func(c *Config) {
		c.Exception = exception
		c.ParseExceptionList()
	}
}

func WithExample(example string) Option {
	return func(c *Config) { c.Example = example }
}

func WithLocal(local bool) Option {
	return func(c *Config) { c.Local = local }
}

func NewConfig(opts ...Option) *Config {
	config := &Config{}
	for _, opt := range opts {
		opt(config)
	}
	return config
}

func init() {
	globalConfig = &Config{}
	flag.BoolVar(&globalConfig.SkipDestroy, "skip-destroy", false, "Skip running terraform destroy after apply")
	flag.StringVar(&globalConfig.Exception, "exception", "", "Comma-separated list of examples to exclude")
	flag.StringVar(&globalConfig.Example, "example", "", "Specific example(s) to test (comma-separated)")
	flag.BoolVar(&globalConfig.Local, "local", false, "Use local source for testing")
	flag.StringVar(&globalConfig.Namespace, "namespace", "cloudnationhq", "Terraform registry namespace")
}

func GetConfig() *Config {
	return globalConfig
}

func (c *Config) ParseExceptionList() {
	c.ExceptionList = []string{}
	if c.Exception == "" {
		return
	}
	for _, ex := range strings.FieldsFunc(c.Exception, func(r rune) bool { return r == ',' }) {
		c.ExceptionList = append(c.ExceptionList, strings.TrimSpace(ex))
	}
}

func TestApplyNoError(t *testing.T) {
	config := setupConfig()
	if config.Example == "" {
		t.Fatal(redError("-example flag is not set"))
	}
	modules := createModulesFromNames(parseExampleList(config.Example), filepath.Join("..", "examples"))
	sourceType := map[bool]string{true: "local", false: "registry"}[config.Local]
	var setup TestSetupFunc
	if config.Local {
		setup = createLocalSetupFunc(config)
	}
	runModuleTests(t, modules, true, config, setup, sourceType)
}

func TestApplyAllParallel(t *testing.T) {
	config := setupConfig()
	modules := discoverModules(t, config)
	RunTests(t, modules, true, config)
}

func TestApplyAllSequential(t *testing.T) {
	config := setupConfig()
	modules := discoverModules(t, config)
	RunTests(t, modules, false, config)
}

func TestApplyAllLocal(t *testing.T) {
	config := setupConfig()
	modules := discoverModules(t, config)
	runModuleTests(t, modules, true, config, createLocalSetupFunc(config), "local")
}

type TestOption func(*TestConfig)

type TestSetupFunc func(ctx context.Context, t *testing.T, modules []*Module) error

type TestConfig struct {
	Config      *Config
	ModuleNames []string
	UseLocal    bool
	Parallel    bool
}

func WithConfig(config *Config) TestOption {
	return func(tc *TestConfig) { tc.Config = config }
}

func WithModules(moduleNames []string) TestOption {
	return func(tc *TestConfig) { tc.ModuleNames = moduleNames }
}

func WithLocalSource(useLocal bool) TestOption {
	return func(tc *TestConfig) { tc.UseLocal = useLocal }
}

func WithParallel(parallel bool) TestOption {
	return func(tc *TestConfig) { tc.Parallel = parallel }
}

func RunTestsWithOptions(t *testing.T, opts ...TestOption) {
	tc := &TestConfig{
		Parallel: true,
	}

	for _, opt := range opts {
		opt(tc)
	}

	if tc.Config == nil {
		tc.Config = GetConfig()
		tc.Config.ParseExceptionList()
	}

	modules := createModulesFromNames(tc.ModuleNames, filepath.Join("..", "examples"))
	sourceType := map[bool]string{true: "local", false: "registry"}[tc.UseLocal]
	var setup TestSetupFunc
	if tc.UseLocal {
		setup = createLocalSetupFunc(tc.Config)
	}
	runModuleTests(t, modules, tc.Parallel, tc.Config, setup, sourceType)
}

func runModuleTests(t *testing.T, modules []*Module, parallel bool, config *Config, setup TestSetupFunc, sourceType string) {
	ctx := context.Background()
	results := NewTestResults()

	if setup != nil {
		if err := setup(ctx, t, modules); err != nil {
			t.Fatal(redError(fmt.Sprintf("Setup failed: %v", err)))
		}
	}

	for _, module := range modules {
		if slices.Contains(config.ExceptionList, module.Name) {
			t.Logf("Skipping example %s as it is in the exception list", module.Name)
			continue
		}

		t.Run(module.Name, func(t *testing.T) {
			if parallel {
				t.Parallel()
			}

			if err := module.Apply(ctx, t); err != nil {
				t.Fail()
			} else {
				t.Logf("✓ Module %s applied successfully with %s source", module.Name, sourceType)
			}

			if !config.SkipDestroy {
				if err := module.Destroy(ctx, t); err != nil && !module.ApplyFailed {
					t.Logf("Cleanup failed for module %s: %v", module.Name, err)
				}
			}

			results.AddModule(module)
		})
	}

	t.Cleanup(func() {
		modules, _ := results.GetResults()
		PrintModuleSummary(t, modules)
	})
}

func setupConfig() *Config {
	config := GetConfig()
	config.ParseExceptionList()
	return config
}

func discoverModules(t *testing.T, config *Config) []*Module {
	manager := NewModuleManager(filepath.Join("..", "examples"))
	manager.SetConfig(config)
	modules, err := manager.DiscoverModules()
	if err != nil {
		errText := fmt.Sprintf("Failed to discover modules: %v", err)
		t.Fatal(redError(errText))
	}
	return modules
}

func extractModuleNames(modules []*Module) []string {
	var moduleNames []string
	for _, module := range modules {
		moduleNames = append(moduleNames, module.Name)
	}
	return moduleNames
}

func createModulesFromNames(moduleNames []string, basePath string) []*Module {
	var modules []*Module
	for _, name := range moduleNames {
		path := filepath.Join(basePath, name)
		modules = append(modules, NewModule(name, path))
	}
	return modules
}

func convertModulesToLocal(ctx context.Context, t *testing.T, converter SourceConverter, moduleNames []string, exceptionList []string, moduleInfo ModuleInfo) []FileRestore {
	var allFilesToRestore []FileRestore

	for _, moduleName := range moduleNames {
		if slices.Contains(exceptionList, moduleName) {
			continue
		}

		modulePath := filepath.Join("..", "examples", moduleName)
		filesToRestore, err := converter.ConvertToLocal(ctx, modulePath, moduleInfo)
		if err != nil {
			t.Logf("Warning: Failed to convert module %s to local source: %v", moduleName, err)
			continue
		}
		allFilesToRestore = append(allFilesToRestore, filesToRestore...)
	}

	return allFilesToRestore
}

func createLocalSetupFunc(config *Config) TestSetupFunc {
	return func(ctx context.Context, t *testing.T, modules []*Module) error {
		moduleInfo := extractModuleInfoFromRepo()
		if moduleInfo.Name == "" || moduleInfo.Provider == "" {
			return fmt.Errorf("could not determine module name and provider from repository")
		}
		moduleInfo.Namespace = config.Namespace

		converter := NewSourceConverter(NewRegistryClient())
		moduleNames := extractModuleNames(modules)
		allFilesToRestore := convertModulesToLocal(ctx, t, converter, moduleNames, config.ExceptionList, moduleInfo)

		t.Cleanup(func() {
			if err := converter.RevertToRegistry(context.Background(), allFilesToRestore); err != nil {
				t.Logf("Warning: Failed to revert files to registry source: %v", err)
			}
		})
		return nil
	}
}

func parseExampleList(example string) []string {
	var examples []string
	for ex := range strings.SplitSeq(example, ",") {
		if trimmed := strings.TrimSpace(ex); trimmed != "" {
			examples = append(examples, trimmed)
		}
	}
	return examples
}

func extractModuleInfoFromRepo() ModuleInfo {
	wd, err := os.Getwd()
	if err != nil {
		return ModuleInfo{}
	}

	if filepath.Base(wd) == "tests" {
		wd = filepath.Dir(wd)
	}

	if repoName := getRepoNameFromGit(wd); repoName != "" {
		re := regexp.MustCompile(`^terraform-([^-]+)-(.+)$`)
		if matches := re.FindStringSubmatch(repoName); len(matches) > 2 {
			return ModuleInfo{
				Name:     matches[2],
				Provider: matches[1],
			}
		}
	}

	repoName := filepath.Base(wd)
	re := regexp.MustCompile(`^terraform-([^-]+)-(.+)$`)
	if matches := re.FindStringSubmatch(repoName); len(matches) > 2 {
		return ModuleInfo{
			Name:     matches[2],
			Provider: matches[1],
		}
	}
	return ModuleInfo{}
}

func getRepoNameFromGit(dir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(string(output))
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		repoName := parts[len(parts)-1]
		return strings.TrimSuffix(repoName, ".git")
	}
	return ""
}
