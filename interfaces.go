package validor

import (
	"context"
	"testing"
)

type ModuleRunner interface {
	Apply(ctx context.Context, t *testing.T) error
	Destroy(ctx context.Context, t *testing.T) error
	Cleanup(ctx context.Context, t *testing.T) error
}

type ModuleDiscoverer interface {
	DiscoverModules(ctx context.Context) ([]ModuleRunner, error)
	SetConfig(config *Config)
}

type SourceConverter interface {
	ConvertToLocal(ctx context.Context, modulePath string, moduleInfo ModuleInfo) ([]FileRestore, error)
	RevertToRegistry(ctx context.Context, filesToRestore []FileRestore) error
}

type RegistryClient interface {
	GetLatestVersion(ctx context.Context, namespace, name, provider string) (string, error)
}

type TestRunner interface {
	RunTests(ctx context.Context, t *testing.T, modules []ModuleRunner, parallel bool, config *Config)
	RunLocalTests(ctx context.Context, t *testing.T, examplesPath string) error
}
