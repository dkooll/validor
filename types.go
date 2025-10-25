package validor

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

type ModuleProcessor interface {
	Apply(ctx context.Context, t *testing.T) error
	Destroy(ctx context.Context, t *testing.T) error
	CleanupFiles(t *testing.T) error
}

type TestResults struct {
	mu            sync.RWMutex
	modules       []*Module
	failedModules []*Module
}

func NewTestResults() *TestResults {
	return &TestResults{
		modules:       make([]*Module, 0),
		failedModules: make([]*Module, 0),
	}
}

func (tr *TestResults) AddModule(module *Module) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.modules = append(tr.modules, module)
	if len(module.Errors) > 0 {
		tr.failedModules = append(tr.failedModules, module)
	}
}

func (tr *TestResults) GetResults() ([]*Module, []*Module) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.modules, tr.failedModules
}

type ModuleInfo struct {
	Name      string
	Provider  string
	Namespace string
}

type FileRestore struct {
	Path            string
	OriginalContent string
	ModuleName      string
	Provider        string
	Namespace       string
}

type TerraformRegistryResponse struct {
	Versions []struct {
		Version string `json:"version"`
	} `json:"versions"`
}

type ModuleError struct {
	ModuleName string
	Operation  string
	Err        error
}

func (e *ModuleError) Error() string {
	return fmt.Sprintf("%s failed for module %s: %v", e.Operation, e.ModuleName, e.Err)
}

func (e *ModuleError) Unwrap() error {
	return e.Err
}
