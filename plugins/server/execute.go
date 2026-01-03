package main

import (
	"encoding/binary"
	"encoding/json"

	"tgp/shared"
)

//go:wasmexport execute
//nolint:unused // Экспортируется через WASM
func execute(ptr uint32, size uint32) uint64 {
	resultPtr, resultSize, hasError := shared.ExecuteWrapper(ptr, size, Free)
	if hasError {
		return (uint64(resultPtr) << 32) | uint64(resultSize) | (1 << 31)
	}
	return (uint64(resultPtr) << 32) | uint64(resultSize)
}

//go:wasmexport info
//nolint:unused // Экспортируется через WASM
func info(ptrPtr uint32, sizePtr uint32) {
	info := pluginInstance.Info()
	infoBytes, _ := json.Marshal(info)

	// Выделяем память через Malloc, чтобы данные не были освобождены сборщиком мусора
	// Проверяем переполнение при конвертации int -> uint32
	if len(infoBytes) > int(^uint32(0)) {
		// Размер слишком большой для uint32, это не должно произойти в реальности
		return
	}
	//nolint:gosec // Проверка переполнения выполнена выше
	resultPtr := Malloc(uint32(len(infoBytes)))
	//nolint:gosec // Проверка переполнения выполнена выше
	resultSize := uint32(len(infoBytes))

	// Копируем данные в выделенную память
	resultMem := PtrToByte(resultPtr, resultSize)
	copy(resultMem, infoBytes)

	// Записываем указатель (uint32, little-endian)
	ptrMem := PtrToByte(ptrPtr, 4)
	binary.LittleEndian.PutUint32(ptrMem, resultPtr)

	// Записываем размер (uint32, little-endian)
	sizeMem := PtrToByte(sizePtr, 4)
	binary.LittleEndian.PutUint32(sizeMem, resultSize)
}

// _initialize автоматически экспортируется при -buildmode=c-shared и вызывается хостом.
//
//nolint:unused // Экспортируется автоматически при -buildmode=c-shared
func _initialize() {}

func main() {}
