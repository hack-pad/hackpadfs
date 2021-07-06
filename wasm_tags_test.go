// +build !wasm

package hackpadfs

import (
	"bufio"
	"go/build/constraint"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWasmTags(t *testing.T) {
	walkErr := filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
		switch {
		case err != nil:
			return err
		case !(strings.HasPrefix(filepath.FromSlash(path), "os/") && strings.HasSuffix(path, "_test.go")):
			// For now, only scan os/*_test.go files. May expand later to other packages and tests that call unimplemented JS functions.
			return nil
		case info.IsDir():
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "//") {
				// hit non-comment line, so no build tags exist (see https://golang.org/cmd/go/#hdr-Build_constraints)
				t.Errorf("File %q does not contain a !wasm build tag", path)
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
			if !isWasm {
				break
			}
		}
		return scanner.Err()
	})
	if walkErr != nil {
		t.Error("Walk failed:", walkErr)
	}
}
