//go:build wasm
// +build wasm

package jswrapper

import "syscall/js"

// Wrapper is implemented by types that are backed by a JavaScript value.
type Wrapper interface {
	// JSValue returns a JavaScript value associated with an object.
	JSValue() js.Value
}
