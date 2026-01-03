package logger

import (
	"context"
	"io"
	"log/slog"

	"github.com/rs/zerolog"
)

// Handler реализует slog.Handler используя zerolog в качестве backend
type Handler struct {
	logger zerolog.Logger
	level  slog.Level
}

// New создает новый slog.Handler с zerolog backend
func New(w io.Writer) *Handler {
	logger := zerolog.New(w).With().Timestamp().Logger()
	return &Handler{
		logger: logger,
		level:  slog.LevelInfo,
	}
}

// NewWithLogger создает новый slog.Handler из существующего zerolog.Logger
func NewWithLogger(logger zerolog.Logger) *Handler {
	return &Handler{
		logger: logger,
		level:  slogLevel(logger.GetLevel()),
	}
}

// Enabled проверяет, включен ли указанный уровень логирования
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle обрабатывает запись лога
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	logEvent := func(event *zerolog.Event) {
		if event == nil {
			return
		}

		event.Time("time", record.Time)

		record.Attrs(func(a slog.Attr) bool {
			addAttr(event, a)
			return true
		})

		event.Msg(record.Message)
	}

	switch level := zerologLevel(record.Level); level {
	case zerolog.ErrorLevel:
		logEvent(h.logger.Error())
	case zerolog.WarnLevel:
		logEvent(h.logger.Warn())
	case zerolog.InfoLevel:
		logEvent(h.logger.Info())
	case zerolog.DebugLevel:
		logEvent(h.logger.Debug())
	default:
		logEvent(h.logger.Trace())
	}
	return nil
}

// WithAttrs возвращает новый Handler с добавленными атрибутами
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	ctx := h.logger.With()
	for _, attr := range attrs {
		ctx = addAttrToContext(ctx, attr)
	}
	return &Handler{
		logger: ctx.Logger(),
		level:  h.level,
	}
}

// WithGroup возвращает новый Handler с группой атрибутов
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		logger: h.logger.With().Str("group", name).Logger(),
		level:  h.level,
	}
}

// zerologLevel преобразует slog.Level в zerolog.Level
func zerologLevel(level slog.Level) zerolog.Level {
	switch {
	case level >= slog.LevelError:
		return zerolog.ErrorLevel
	case level >= slog.LevelWarn:
		return zerolog.WarnLevel
	case level >= slog.LevelInfo:
		return zerolog.InfoLevel
	case level >= slog.LevelDebug:
		return zerolog.DebugLevel
	default:
		return zerolog.TraceLevel
	}
}

// slogLevel преобразует zerolog.Level в slog.Level
func slogLevel(level zerolog.Level) slog.Level {
	switch level {
	case zerolog.Disabled:
		return slog.Level(999) // Максимальный уровень, чтобы отключить логирование
	case zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel:
		return slog.LevelError
	case zerolog.WarnLevel:
		return slog.LevelWarn
	case zerolog.InfoLevel:
		return slog.LevelInfo
	case zerolog.DebugLevel:
		return slog.LevelDebug
	case zerolog.TraceLevel:
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

// addAttr добавляет атрибут slog в zerolog event
func addAttr(event *zerolog.Event, attr slog.Attr) *zerolog.Event {
	key := attr.Key
	value := attr.Value

	switch value.Kind() {
	case slog.KindString:
		return event.Str(key, value.String())
	case slog.KindInt64:
		return event.Int64(key, value.Int64())
	case slog.KindUint64:
		return event.Uint64(key, value.Uint64())
	case slog.KindFloat64:
		return event.Float64(key, value.Float64())
	case slog.KindBool:
		return event.Bool(key, value.Bool())
	case slog.KindDuration:
		return event.Dur(key, value.Duration())
	case slog.KindTime:
		return event.Time(key, value.Time())
	case slog.KindAny:
		return event.Interface(key, value.Any())
	default:
		return event.Interface(key, value.Any())
	}
}

// addAttrToContext добавляет атрибут slog в zerolog context
func addAttrToContext(ctx zerolog.Context, attr slog.Attr) zerolog.Context {
	key := attr.Key
	value := attr.Value

	switch value.Kind() {
	case slog.KindString:
		return ctx.Str(key, value.String())
	case slog.KindInt64:
		return ctx.Int64(key, value.Int64())
	case slog.KindUint64:
		return ctx.Uint64(key, value.Uint64())
	case slog.KindFloat64:
		return ctx.Float64(key, value.Float64())
	case slog.KindBool:
		return ctx.Bool(key, value.Bool())
	case slog.KindDuration:
		return ctx.Dur(key, value.Duration())
	case slog.KindTime:
		return ctx.Time(key, value.Time())
	case slog.KindAny:
		return ctx.Interface(key, value.Any())
	default:
		return ctx.Interface(key, value.Any())
	}
}

// SetLevel обновляет минимальный уровень логирования для slog.Logger
func SetLevel(logger *slog.Logger, level slog.Level) {
	if handler, ok := logger.Handler().(*Handler); ok {
		handler.SetLevel(level)
	}
}

// SetLevel устанавливает минимальный уровень логирования
func (h *Handler) SetLevel(level slog.Level) {
	h.level = level
}

// Logger возвращает базовый zerolog.Logger
func (h *Handler) Logger() zerolog.Logger {
	return h.logger
}

// NewLogger создает новый slog.Logger с zerolog backend
func NewLogger(w io.Writer) *slog.Logger {
	return slog.New(New(w))
}

// NewZerolog создает новый slog.Logger из существующего zerolog.Logger
func NewZerolog(logger zerolog.Logger) *slog.Logger {
	return slog.New(NewWithLogger(logger))
}
