package validor

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultRegistryClient_GetLatestVersion(t *testing.T) {
	client := NewRegistryClient().(*DefaultRegistryClient)
	client.client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"versions":[{"version":"2.0.0"},{"version":"1.0.0"}]}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	version, err := client.GetLatestVersion(context.Background(), "ns", "name", "provider")
	if err != nil {
		t.Fatalf("GetLatestVersion returned error: %v", err)
	}
	if version != "2.0.0" {
		t.Fatalf("expected 2.0.0, got %s", version)
	}
}

func TestDefaultRegistryClient_GetLatestVersion_Errors(t *testing.T) {
	t.Run("non-200 response", func(t *testing.T) {
		client := NewRegistryClient().(*DefaultRegistryClient)
		client.client = &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				}, nil
			}),
		}

		if _, err := client.GetLatestVersion(context.Background(), "ns", "name", "provider"); err == nil {
			t.Fatalf("expected error for non-200 response")
		}
	})

	t.Run("empty versions", func(t *testing.T) {
		client := NewRegistryClient().(*DefaultRegistryClient)
		client.client = &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"versions":[]}`)),
					Header:     make(http.Header),
				}, nil
			}),
		}

		if _, err := client.GetLatestVersion(context.Background(), "ns", "name", "provider"); err == nil {
			t.Fatalf("expected error when no versions are returned")
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		client := NewRegistryClient().(*DefaultRegistryClient)
		client.client = &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{invalid`)),
					Header:     make(http.Header),
				}, nil
			}),
		}

		if _, err := client.GetLatestVersion(context.Background(), "ns", "name", "provider"); err == nil {
			t.Fatalf("expected error for malformed json")
		}
	})
}

func TestDefaultSourceConverter_RevertToRegistry_Fallback(t *testing.T) {
	tmpDir := t.TempDir()
	tfFile := filepath.Join(tmpDir, "main.tf")

	originalContent := `module "test" {
  source  = "cloudnationhq/mymodule/azure"
  version = "~> 1.0"
}`

	if err := os.WriteFile(tfFile, []byte("local override"), 0o644); err != nil {
		t.Fatalf("failed to write tf file: %v", err)
	}

	converter := NewSourceConverter(&mockRegistryClient{err: errors.New("boom")})
	filesToRestore := []FileRestore{{
		Path:            tfFile,
		OriginalContent: originalContent,
		ModuleName:      "mymodule",
		Provider:        "azure",
		Namespace:       "cloudnationhq",
	}}

	if err := converter.RevertToRegistry(context.Background(), filesToRestore); err != nil {
		t.Fatalf("RevertToRegistry returned error: %v", err)
	}

	content, err := os.ReadFile(tfFile)
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}

	if string(content) != originalContent {
		t.Fatalf("expected file to be restored to original content, got: %s", string(content))
	}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
