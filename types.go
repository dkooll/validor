package validor

import (
	"testing"
)

// ModuleProcessor applies, destroys, and cleans up Terraform modules
type ModuleProcessor interface {
	Apply(t *testing.T) error
	Destroy(t *testing.T) error
	CleanupFiles(t *testing.T) error
}

// ModuleDiscoverer finds modules within a directory structure
type ModuleDiscoverer interface {
	DiscoverModules() ([]*Module, error)
}

// TestRunner executes tests for Terraform modules
type TestRunner interface {
	RunTests(t *testing.T, modules []*Module, parallel bool)
}

// Logger provides formatted logging capabilities
type Logger interface {
	Logf(format string, args ...any)
}

// SimpleLogger is a no-op implementation of the Logger interface
type SimpleLogger struct{}

// Logf is a no-op implementation that does nothing
func (l *SimpleLogger) Logf(format string, args ...any) {
	// No-op implementation
}
