// +build js,wasm

package js

import (
	"syscall/js"
	"testing"
)

func TestJS(t *testing.T) {
	js.Global().Set("woot", true)
}
