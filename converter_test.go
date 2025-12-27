package validor

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type mockRegistryClient struct {
	latestVersion string
	err           error
}

func (m *mockRegistryClient) GetLatestVersion(ctx context.Context, namespace, name, provider string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.latestVersion, nil
}

func TestNewSourceConverter(t *testing.T) {
	client := NewRegistryClient()
	converter := NewSourceConverter(client)

	if converter == nil {
		t.Error("NewSourceConverter() should not return nil")
	}

	if _, ok := converter.(*DefaultSourceConverter); !ok {
		t.Error("NewSourceConverter() should return *DefaultSourceConverter")
	}
}

func TestDefaultSourceConverter_ConvertToLocal(t *testing.T) {
	tmpDir := t.TempDir()

	tfContent := `
module "test" {
  source  = "cloudnationhq/mymodule/azure"
  version = "~> 1.0"
}

module "submodule" {
  source  = "cloudnationhq/mymodule/azure//modules/network"
  version = "~> 1.0"
}
`
	tfFile := filepath.Join(tmpDir, "main.tf")
	if err := os.WriteFile(tfFile, []byte(tfContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	client := &mockRegistryClient{latestVersion: "1.0.0"}
	converter := NewSourceConverter(client)

	moduleInfo := ModuleInfo{
		Name:      "mymodule",
		Provider:  "azure",
		Namespace: "cloudnationhq",
	}

	ctx := testContext(t)
	filesToRestore, err := converter.ConvertToLocal(ctx, tmpDir, moduleInfo)
	if err != nil {
		t.Errorf("ConvertToLocal() error = %v", err)
	}

	if len(filesToRestore) == 0 {
		t.Error("ConvertToLocal() should have files to restore")
	}

	content, err := os.ReadFile(tfFile)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	contentStr := string(content)
	if !regexp.MustCompile(`source\s*=\s*"../../"`).MatchString(contentStr) {
		t.Error("Main module source should be converted to local path")
	}
	if !regexp.MustCompile(`source\s*=\s*"../../modules/network"`).MatchString(contentStr) {
		t.Error("Submodule source should be converted to local path")
	}
}

func TestDefaultSourceConverter_RevertToRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	tfFile := filepath.Join(tmpDir, "main.tf")

	originalContent := `
module "test" {
  source  = "cloudnationhq/mymodule/azure"
  version = "~> 1.0"
}
`

	modifiedContent := `
module "test" {
  source = "../../"
}
`

	if err := os.WriteFile(tfFile, []byte(modifiedContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	client := &mockRegistryClient{latestVersion: "1.5.0"}
	converter := NewSourceConverter(client)

	filesToRestore := []FileRestore{
		{
			Path:            tfFile,
			OriginalContent: originalContent,
			ModuleName:      "mymodule",
			Provider:        "azure",
			Namespace:       "cloudnationhq",
		},
	}

	ctx := testContext(t)
	err := converter.RevertToRegistry(ctx, filesToRestore)
	if err != nil {
		t.Errorf("RevertToRegistry() error = %v", err)
	}

	content, err := os.ReadFile(tfFile)
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}

	contentStr := string(content)
	if !regexp.MustCompile(`version\s*=\s*"~>\s*1\.5\.0"`).MatchString(contentStr) {
		t.Errorf("Version should be updated to latest (1.5.0), got: %s", contentStr)
	}
}

func TestDefaultSourceConverter_updateVersionInContent(t *testing.T) {
	client := &mockRegistryClient{}
	converter := NewSourceConverter(client).(*DefaultSourceConverter)

	tests := []struct {
		name          string
		content       string
		latestVersion string
		expectedMatch string
	}{
		{
			name: "update existing version",
			content: `module "test" {
  source  = "cloudnationhq/mymodule/azure"
  version = "~> 1.0"
}`,
			latestVersion: "2.0.0",
			expectedMatch: `version = "~> 2.0.0"`,
		},
		{
			name: "no version attribute",
			content: `module "test" {
  source = "../../"
}`,
			latestVersion: "2.0.0",
			expectedMatch: "",
		},
		{
			name: "version with different format",
			content: `module "test" {
  version="1.0.0"
}`,
			latestVersion: "3.0.0",
			expectedMatch: `version="~> 3.0.0"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.updateVersionInContent(tt.content, tt.latestVersion)

			if tt.expectedMatch != "" {
				matched, _ := regexp.MatchString(regexp.QuoteMeta(tt.expectedMatch), result)
				if !matched {
					t.Errorf("Expected content to contain %q, got: %s", tt.expectedMatch, result)
				}
			} else {
				if result != tt.content {
					t.Errorf("Content should remain unchanged when no version attribute exists")
				}
			}
		})
	}
}

func TestDefaultSourceConverter_ConvertToLocal_CancelledMidFile(t *testing.T) {
	tmpDir := t.TempDir()
	tfContent := `
module "one" {
  source  = "cloudnationhq/mymodule/azure"
  version = "~> 1.0"
}

module "two" {
  source  = "cloudnationhq/mymodule/azure//modules/net"
  version = "~> 1.0"
}
`
	tfFile := filepath.Join(tmpDir, "main.tf")
	if err := os.WriteFile(tfFile, []byte(tfContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	converter := NewSourceConverter(&mockRegistryClient{latestVersion: "1.0.0"})
	moduleInfo := ModuleInfo{
		Name:      "mymodule",
		Provider:  "azure",
		Namespace: "cloudnationhq",
	}

	filesToRestore, err := converter.ConvertToLocal(ctx, tmpDir, moduleInfo)
	if err == nil && len(filesToRestore) > 0 {
		t.Fatalf("expected no changes when context is cancelled, got %v", filesToRestore)
	}
	if err == nil {
		t.Fatalf("expected cancellation error")
	}

	content, _ := os.ReadFile(tfFile)
	if string(content) != tfContent {
		t.Fatalf("file should remain unchanged when context is cancelled")
	}
}

func TestDefaultSourceConverter_updateModuleBlock(t *testing.T) {
	client := &mockRegistryClient{}
	converter := NewSourceConverter(client).(*DefaultSourceConverter)

	moduleSource := "cloudnationhq/mymodule/azure"
	submoduleRegex := regexp.MustCompile(`^cloudnationhq/mymodule/azure//modules/(.*)$`)

	tests := []struct {
		name           string
		sourceValue    string
		expectedSource string
		shouldChange   bool
	}{
		{
			name:           "main module source",
			sourceValue:    "cloudnationhq/mymodule/azure",
			expectedSource: "../../",
			shouldChange:   true,
		},
		{
			name:           "submodule source",
			sourceValue:    "cloudnationhq/mymodule/azure//modules/network",
			expectedSource: "../../modules/network",
			shouldChange:   true,
		},
		{
			name:           "different module source",
			sourceValue:    "hashicorp/consul/aws",
			expectedSource: "hashicorp/consul/aws",
			shouldChange:   false,
		},
		{
			name:           "local source",
			sourceValue:    "../../",
			expectedSource: "../../",
			shouldChange:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := hclwrite.NewEmptyFile()
			rootBody := f.Body()
			block := rootBody.AppendNewBlock("module", []string{"test"})
			block.Body().SetAttributeValue("source", cty.StringVal(tt.sourceValue))

			changed := converter.updateModuleBlock(block, moduleSource, submoduleRegex)

			if changed != tt.shouldChange {
				t.Errorf("updateModuleBlock() changed = %v, want %v", changed, tt.shouldChange)
			}

			attr := block.Body().GetAttribute("source")
			if attr != nil {
				sourceVal, ok := attributeStringValue(attr)
				if ok && sourceVal != tt.expectedSource {
					t.Errorf("Source value = %v, want %v", sourceVal, tt.expectedSource)
				}
			}
		})
	}
}

func TestAttributeStringValue(t *testing.T) {
	tests := []struct {
		name        string
		value       cty.Value
		wantValue   string
		wantSuccess bool
	}{
		{
			name:        "valid string",
			value:       cty.StringVal("test-value"),
			wantValue:   "test-value",
			wantSuccess: true,
		},
		{
			name:        "string with path",
			value:       cty.StringVal("../../modules/test"),
			wantValue:   "../../modules/test",
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := hclwrite.NewEmptyFile()
			rootBody := f.Body()
			rootBody.SetAttributeValue("test", tt.value)
			attr := rootBody.GetAttribute("test")

			value, ok := attributeStringValue(attr)

			if ok != tt.wantSuccess {
				t.Errorf("attributeStringValue() ok = %v, want %v", ok, tt.wantSuccess)
			}
			if ok && value != tt.wantValue {
				t.Errorf("attributeStringValue() value = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}
