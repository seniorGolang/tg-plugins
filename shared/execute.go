package shared

import (
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
		resultPtr, resultSize := ByteToPtr(errorBytes)
		return resultPtr, resultSize, true
	}

	// Преобразуем project из map обратно в Project
	var project Project
	if projectMap, ok := req.Project.(map[string]any); ok {
		projectBytes, _ := json.Marshal(projectMap)
		_ = json.Unmarshal(projectBytes, &project)
	}

	// Вызываем метод Execute плагина
	if pluginInstance == nil {
		errorBytes, _ := json.Marshal(ExecuteResponse{Error: "plugin instance not set"})
		resultPtr, resultSize := ByteToPtr(errorBytes)
		return resultPtr, resultSize, true
	}
	err := pluginInstance.Execute(project, req.RootDir, req.Options, req.Path...)

	// Формируем ответ
	var resp ExecuteResponse
	if err != nil {
		resp.Error = err.Error()
	}

	responseBytes, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		errorBytes, _ := json.Marshal(ExecuteResponse{Error: "failed to marshal response: " + marshalErr.Error()})
		resultPtr, resultSize := ByteToPtr(errorBytes)
		return resultPtr, resultSize, true
	}

	resultPtr, resultSize = ByteToPtr(responseBytes)
	if err != nil {
		return resultPtr, resultSize, true
	}
	return resultPtr, resultSize, false
}
