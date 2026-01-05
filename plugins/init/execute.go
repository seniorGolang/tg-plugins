package main

import (
	"tgp/core"
)

//go:wasmexport execute
//nolint:unused // Экспортируется через WASM
func execute(ptr uint32, size uint32) uint64 {
	resultPtr, resultSize, hasError := core.ExecuteWrapper(ptr, size, Free)
	if hasError {
		return (uint64(resultPtr) << 32) | uint64(resultSize) | (1 << 31)
	}
	return (uint64(resultPtr) << 32) | uint64(resultSize)
}

//go:wasmexport info
//nolint:unused // Экспортируется через WASM
func info(ptrPtr uint32, sizePtr uint32) {
	core.InfoWrapper(ptrPtr, sizePtr)
}

// _initialize автоматически экспортируется при -buildmode=c-shared и вызывается хостом.
//
//nolint:unused // Экспортируется автоматически при -buildmode=c-shared
func _initialize() {}

func main() {}
