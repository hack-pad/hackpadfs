//go:build !wasm
// +build !wasm

package hackpadfs

import (
	"bufio"
	"go/build/constraint"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hack-pad/hackpadfs/internal/assert"
)

var (
	noWasm = []string{
		"os/*_test.go",
	}
	yesWasm = []string{
		"indexeddb/*",
	}
)

func shouldBeWasm(filePath string) (isWasm, skip bool) {
	if path.Ext(filePath) != ".go" {
		return false, true
	}
	for _, glob := range yesWasm {
		if match, _ := filepath.Match(glob, filePath); match {
			return true, false
		}
	}
	for _, glob := range noWasm {
		if match, _ := filepath.Match(glob, filePath); match {
			return false, false
		}
	}
	return false, true
}

func TestWasmTags(t *testing.T) {
	t.Parallel()
	walkErr := filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		shouldBeWasm, skip := shouldBeWasm(path)
		if skip {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { assert.NoError(t, f.Close()) }()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "//") {
				// hit non-comment line, so no build tags exist (see https://golang.org/cmd/go/#hdr-Build_constraints)
				tag := "wasm"
				if !shouldBeWasm {
					tag = "!wasm"
				}
				t.Errorf("File %q does not contain a %s build tag", path, tag)
				break
			}

			expr, err := constraint.Parse(line)
			if err != nil {
				t.Logf("Build constraint failed to parse line in file %q: %q; %v", path, line, err)
				continue
			}
			isWasm := expr.Eval(func(tag string) bool {
				return tag == "wasm"
			})
			if isWasm == shouldBeWasm {
				break
			}
		}
		return scanner.Err()
	})
	if walkErr != nil {
		t.Error("Walk failed:", walkErr)
	}
}
