//go:build wasip1

package core

// Импорты функций хоста.
// Функции объявлены без тела - их реализация предоставляется хостом во время выполнения WASM.
// Это валидный Go код при компиляции для GOOS=wasip1 GOARCH=wasm.

//go:wasmimport env log_debug
func hostLogDebug(msgPtr uint32, msgLen uint32)

//go:wasmimport env log_info
func hostLogInfo(msgPtr uint32, msgLen uint32)

//go:wasmimport env log_warn
func hostLogWarn(msgPtr uint32, msgLen uint32)

//go:wasmimport env log_error
func hostLogError(msgPtr uint32, msgLen uint32)

//go:wasmimport env http_request
func hostHTTPRequest(methodPtr, methodLen, urlPtr, urlLen, headersPtr, headersLen, bodyPtr, bodyLen, resultPtrPtr, resultSizePtr uint32) uint32

//go:wasmimport env host_execute_command
func hostExecuteCommand(commandPtr, commandLen, argsPtr, argsLen, workDirPtr, workDirLen, resultPtrPtr, resultSizePtr uint32) uint32

// hostLoggerAdapter адаптирует функции хоста к интерфейсу Logger.
type hostLoggerAdapter struct{}

func (a *hostLoggerAdapter) Debug(msg string) {
	msgPtr, msgLen := ByteToPtr([]byte(msg))
	hostLogDebug(msgPtr, msgLen)
}

func (a *hostLoggerAdapter) Info(msg string) {
	msgPtr, msgLen := ByteToPtr([]byte(msg))
	hostLogInfo(msgPtr, msgLen)
}

func (a *hostLoggerAdapter) Warn(msg string) {
	msgPtr, msgLen := ByteToPtr([]byte(msg))
	hostLogWarn(msgPtr, msgLen)
}

func (a *hostLoggerAdapter) Error(msg string) {
	msgPtr, msgLen := ByteToPtr([]byte(msg))
	hostLogError(msgPtr, msgLen)
}

// hostCommandExecutor адаптирует функции хоста к интерфейсу CommandExecutor.
type hostCommandExecutor struct{}

func (e *hostCommandExecutor) ExecuteCommand(commandPtr, commandLen, argsPtr, argsLen, workDirPtr, workDirLen, resultPtrPtr, resultSizePtr uint32) uint32 {
	return hostExecuteCommand(commandPtr, commandLen, argsPtr, argsLen, workDirPtr, workDirLen, resultPtrPtr, resultSizePtr)
}

// init инициализирует адаптеры.
func init() {
	SetLogger(&hostLoggerAdapter{})
	SetCommandExecutor(&hostCommandExecutor{})
}
