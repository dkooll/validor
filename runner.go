package validor

import (
	"testing"
)

func RunTests(t *testing.T, modules []*Module, parallel bool, config *Config) {
	runModuleTests(t, modules, parallel, config, nil, "registry")
}
