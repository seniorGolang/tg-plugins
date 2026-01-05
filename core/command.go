package core

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// CommandExecutor предоставляет функции выполнения команд.
type CommandExecutor interface {
	ExecuteCommand(commandPtr, commandLen, argsPtr, argsLen, workDirPtr, workDirLen, resultPtrPtr, resultSizePtr uint32) uint32
}

var commandExecutor CommandExecutor

// SetCommandExecutor устанавливает исполнитель команд.
func SetCommandExecutor(ce CommandExecutor) {
	commandExecutor = ce
}

// WriteString выделяет память и записывает строку. Память должна быть освобождена через Free.
func WriteString(s string) (ptr uint32, length uint32) {
	bytes := []byte(s)
	if len(bytes) == 0 {
		return 0, 0
	}
	if len(bytes) > int(^uint32(0)) {
		return 0, 0
	}
	ptr = Malloc(uint32(len(bytes))) //nolint:gosec // Преобразование int -> uint32 безопасно, так как размеры проверяются выше
	if ptr == 0 {
		return 0, 0
	}
	copy(PtrToByte(ptr, uint32(len(bytes))), bytes) //nolint:gosec // Преобразование int -> uint32 безопасно, так как размеры проверяются выше
	length = uint32(len(bytes)) //nolint:gosec // Преобразование int -> uint32 безопасно, так как размеры проверяются выше
	return ptr, length
}

// ReadUint32 читает uint32 из памяти (little-endian, 4 байта).
func ReadUint32(ptr uint32) uint32 {
	bytes := PtrToByte(ptr, 4)
	return binary.LittleEndian.Uint32(bytes)
}

// ExecuteCommandInDir выполняет команду через хост в указанной директории.
// command - имя команды (например, "go")
// args - аргументы команды
// workDir - рабочая директория (относительно rootDir)
// Возвращает результат выполнения команды или ошибку.
func ExecuteCommandInDir(command string, args []string, workDir string) (response *CommandResponse, err error) {
	if commandExecutor == nil {
		return nil, fmt.Errorf("command executor not initialized")
	}

	// Сериализуем args в JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	// Выделяем память для строк
	commandPtr, commandLen := WriteString(command)
	defer Free(commandPtr)

	argsPtr, argsLen := WriteString(string(argsJSON))
	defer Free(argsPtr)

	workDirPtr, workDirLen := WriteString(workDir)
	defer Free(workDirPtr)

	// Выделяем память для указателей результата
	resultPtrPtr := Malloc(4)
	resultSizePtr := Malloc(4)
	defer Free(resultPtrPtr)
	defer Free(resultSizePtr)

	// Вызываем функцию хоста через адаптер
	if commandExecutor.ExecuteCommand(commandPtr, commandLen, argsPtr, argsLen, workDirPtr, workDirLen, resultPtrPtr, resultSizePtr) != 0 {
		return nil, fmt.Errorf("failed to execute command")
	}

	// Читаем указатель и размер результата
	resultPtr := ReadUint32(resultPtrPtr)
	resultSize := ReadUint32(resultSizePtr)

	// Читаем результат
	resultBytes := PtrToByte(resultPtr, resultSize)
	defer Free(resultPtr)

	var resp CommandResponse
	if err := json.Unmarshal(resultBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.Error != "" {
		response = &resp
		return response, fmt.Errorf("command execution failed: %s", resp.Error)
	}

	response = &resp
	return response, nil
}

