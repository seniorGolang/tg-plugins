package core

import "unsafe"

// Управление памятью для WASM плагина.
// Хост использует эти функции для выделения памяти в модуле.
var allocations = make(map[uint32][]byte)

func allocate(size uint32) uint32 {
	if size == 0 {
		return 0
	}
	b := make([]byte, size)
	ptr := uint32(uintptr(unsafe.Pointer(&b[0])))
	allocations[ptr] = b
	return ptr
}

// Malloc выделяет память (используется в плагине через экспорт).
func Malloc(size uint32) uint32 {
	return allocate(size)
}

// Free освобождает память (используется в плагине через экспорт).
func Free(ptr uint32) {
	delete(allocations, ptr)
}

// PtrToByte преобразует указатель и размер в байтовый срез.
func PtrToByte(ptr, size uint32) []byte {
	//nolint:govet // unsafe.Pointer необходим для работы с WASM памятью
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), size)
}

// ByteToPtr преобразует байтовый срез в указатель и размер.
func ByteToPtr(buf []byte) (uint32, uint32) {
	if len(buf) == 0 {
		return 0, 0
	}
	ptr := &buf[0]
	//nolint:gosec // unsafe.Pointer необходим для работы с WASM памятью
	unsafePtr := uintptr(unsafe.Pointer(ptr))
	if unsafePtr > uintptr(^uint32(0)) {
		panic("pointer value too large for uint32")
	}
	if len(buf) > int(^uint32(0)) {
		panic("buffer size too large for uint32")
	}
	return uint32(unsafePtr), uint32(len(buf)) //nolint:gosec // Преобразование int -> uint32 безопасно, так как размеры проверяются выше
}
