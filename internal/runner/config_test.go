package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigEnablesConPTYOnlyForConfiguredEnvironment(t *testing.T) {
	config := Config{ConPTYEnvironments: []HostEnvironment{HostEnvironmentWindows10}}

	assert.True(t, config.enablesConPTY(HostEnvironmentWindows10))
	assert.False(t, Config{}.enablesConPTY(HostEnvironmentWindows10))
	assert.False(t, config.enablesConPTY(HostEnvironmentWindows11))

	windows11 := Config{ConPTYEnvironments: []HostEnvironment{HostEnvironmentWindows11}}
	assert.True(t, windows11.enablesConPTY(HostEnvironmentWindows11))
	assert.False(t, windows11.enablesConPTY(HostEnvironmentWindows10))
}
