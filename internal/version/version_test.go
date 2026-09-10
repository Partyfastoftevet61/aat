package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEffective(t *testing.T) {
	orig := Version
	t.Cleanup(func() { Version = orig })

	Version = "v9.9.9"
	assert.Equal(t, "v9.9.9", Effective(), "ldflags version wins")

	Version = "dev"
	assert.NotEmpty(t, Effective(), "falls back to build info or the dev marker")
}
