package shared

// Logger предоставляет функции логирования.
type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

var logger Logger

// SetLogger устанавливает логгер.
func SetLogger(l Logger) {
	logger = l
}

// GetLogger возвращает текущий логгер.
func GetLogger() Logger {
	return logger
}
