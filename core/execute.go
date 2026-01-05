package core

import (
	"encoding/binary"
	"encoding/json"
)

var pluginInstance Plugin

// SetPluginInstance устанавливает экземпляр плагина.
func SetPluginInstance(p Plugin) {
	pluginInstance = p
}

// ExecuteWrapper выполняет плагин с сериализованными данными.
func ExecuteWrapper(ptr uint32, size uint32, freeFunc func(uint32)) (resultPtr uint32, resultSize uint32, hasError bool) {

// Читаем запрос
	requestBytes := PtrToByte(ptr, size)
	if freeFunc != nil {
		defer freeFunc(ptr)
	}

	var req ExecuteRequest
	if err := json.Unmarshal(requestBytes, &req); err != nil {
		errorBytes, _ := json.Marshal(ExecuteResponse{Error: "failed to unmarshal request: " + err.Error()})
		resultPtr, resultSize = ByteToPtr(errorBytes)
		return resultPtr, resultSize, true
	}

	// Вызываем метод Execute плагина
	if pluginInstance == nil {
		errorBytes, _ := json.Marshal(ExecuteResponse{Error: "plugin instance not set"})
		resultPtr, resultSize = ByteToPtr(errorBytes)
		return resultPtr, resultSize, true
	}
	response, err := pluginInstance.Execute(req.RootDir, req.Request, req.Path...)

	// Формируем ответ
	var resp ExecuteResponse
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Response = response
	}

	responseBytes, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		errorBytes, _ := json.Marshal(ExecuteResponse{Error: "failed to marshal response: " + marshalErr.Error()})
		resultPtr, resultSize = ByteToPtr(errorBytes)
		return resultPtr, resultSize, true
	}

	resultPtr, resultSize = ByteToPtr(responseBytes)
	if err != nil {
		return resultPtr, resultSize, true
	}
	return resultPtr, resultSize, false
}

// InfoWrapper получает информацию о плагине и записывает её в указанные указатели.
// ptrPtr - указатель на uint32, куда будет записан указатель на данные.
// sizePtr - указатель на uint32, куда будет записан размер данных.
func InfoWrapper(ptrPtr uint32, sizePtr uint32) {

	if pluginInstance == nil {
		// Если плагин не установлен, записываем нули
		ptrMem := PtrToByte(ptrPtr, 4)
		sizeMem := PtrToByte(sizePtr, 4)
		binary.LittleEndian.PutUint32(ptrMem, 0)
		binary.LittleEndian.PutUint32(sizeMem, 0)
		return
	}

	info := pluginInstance.Info()
	infoBytes, err := json.Marshal(info)
	if err != nil {
		// При ошибке сериализации записываем нули
		ptrMem := PtrToByte(ptrPtr, 4)
		sizeMem := PtrToByte(sizePtr, 4)
		binary.LittleEndian.PutUint32(ptrMem, 0)
		binary.LittleEndian.PutUint32(sizeMem, 0)
		return
	}

	// Выделяем память через Malloc, чтобы данные не были освобождены сборщиком мусора
	// Проверяем переполнение при конвертации int -> uint32
	if len(infoBytes) > int(^uint32(0)) {
		// Размер слишком большой для uint32, записываем нули
		ptrMem := PtrToByte(ptrPtr, 4)
		sizeMem := PtrToByte(sizePtr, 4)
		binary.LittleEndian.PutUint32(ptrMem, 0)
		binary.LittleEndian.PutUint32(sizeMem, 0)
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
