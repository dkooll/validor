package validor

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type mockTB struct {
	logs  []string
	fatal bool
}

func (m *mockTB) Helper() {}
func (m *mockTB) Log(args ...any) {
	m.logs = append(m.logs, strings.TrimSpace(fmt.Sprint(args...)))
}
func (m *mockTB) Logf(format string, args ...any) {
	m.logs = append(m.logs, fmt.Sprintf(format, args...))
}
func (m *mockTB) Fatal(args ...any) {
	m.fatal = true
	m.logs = append(m.logs, strings.TrimSpace(fmt.Sprint(args...)))
}

func TestRunModuleTests_SkipDestroy(t *testing.T) {
	module := NewModule("mod1", t.TempDir())
	var destroyCalled bool

	module.applyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		return nil
	}
	module.destroyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		destroyCalled = true
		return nil
	}

	config := &Config{SkipDestroy: true}
	runModuleTests(t, []*Module{module}, false, config, nil, "local")

	if destroyCalled {
		t.Fatalf("destroy should not be called when SkipDestroy is true")
	}
}

func TestRunModuleTests_InvokesDestroyWhenAllowed(t *testing.T) {
	module := NewModule("mod1", t.TempDir())
	var destroyCalled bool

	module.applyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		return nil
	}
	module.destroyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		destroyCalled = true
		return nil
	}

	config := &Config{SkipDestroy: false}
	runModuleTests(t, []*Module{module}, false, config, nil, "local")

	if !destroyCalled {
		t.Fatalf("destroy should be called when SkipDestroy is false")
	}
}

func TestRunModuleTests_RespectsExceptionList(t *testing.T) {
	seen := make(map[string]bool)

	mod1 := NewModule("run-me", t.TempDir())
	mod1.applyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		seen[m.Name] = true
		return nil
	}
	mod1.destroyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		return nil
	}
	mod1.cleanupHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		return nil
	}
	mod2 := NewModule("skip-me", t.TempDir())
	mod2.applyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		seen[m.Name] = true
		return nil
	}
	mod2.destroyHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		return nil
	}
	mod2.cleanupHook = func(ctx context.Context, tb *testing.T, m *Module) error {
		return nil
	}

	config := &Config{ExceptionList: []string{"skip-me"}}
	runModuleTests(t, []*Module{mod1, mod2}, false, config, nil, "local")

	if !seen["run-me"] {
		t.Fatalf("expected module run-me to execute")
	}
	if seen["skip-me"] {
		t.Fatalf("expected module skip-me to be skipped")
	}
}

func TestPrintModuleSummary_CapturesOutput(t *testing.T) {
	mock := &mockTB{}
	modules := []*Module{
		{
			Name:   "broken",
			Errors: []string{"terraform apply failed"},
		},
		NewModule("ok", t.TempDir()),
	}

	PrintModuleSummary(mock, modules)

	joined := strings.Join(mock.logs, "\n")
	if !strings.Contains(joined, "broken") || !strings.Contains(joined, "terraform apply failed") {
		t.Fatalf("expected summary to include module name and error, got %q", joined)
	}
	if !strings.Contains(joined, "TOTAL: 1 of 2 modules failed") {
		t.Fatalf("expected failure count in summary, got %q", joined)
	}
}
