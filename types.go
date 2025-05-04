package validor

import (
	"testing"
)

// ModuleProcessor defines methods for processing Terraform modules
type ModuleProcessor interface {
	Apply(t *testing.T) error
	Destroy(t *testing.T) error
	CleanupFiles(t *testing.T) error
}

// ModuleDiscoverer discovers modules within a directory structure
type ModuleDiscoverer interface {
	DiscoverModules() ([]*Module, error)
}

// TestRunner runs tests for Terraform modules
type TestRunner interface {
	RunTests(t *testing.T, modules []*Module, parallel bool)
}

// Logger provides logging capabilities
type Logger interface {
	Logf(format string, args ...any)
}

// SimpleLogger is a basic implementation of the Logger interface
type SimpleLogger struct{}

// Logf implements the Logger interface
func (l *SimpleLogger) Logf(format string, args ...any) {
	// Use t.Logf directly in testing context
}
