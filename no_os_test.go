// +build !wasm

package hackpadfs

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"testing"

	"github.com/hack-pad/hackpadfs/internal/assert"
)

func TestNoImportOS(t *testing.T) {
	// Ensure we don't import the standard library "os" package unnecessarily in the main interface package.
	// This helps reduce footprint, since os is not required unless using the hackpadfs/os implementation.
	cmd := exec.Command("go", "list", "-json", ".")
	output, err := cmd.CombinedOutput()
	assert.NoError(t, err)

	type module struct {
		ImportPath string
		Deps       []string
	}
	decoder := json.NewDecoder(bytes.NewReader(output))

	var listOut []module
	for {
		var m module
		err := decoder.Decode(&m)
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		listOut = append(listOut, m)
	}

	assert.NotZero(t, len(listOut))
	for _, pkg := range listOut {
		if !assert.NotContains(t, pkg.Deps, "os") {
			t.Log("Failed module import path:", pkg.ImportPath)
		}
	}
}
