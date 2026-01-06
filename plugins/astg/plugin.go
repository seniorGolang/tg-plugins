package main

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"tgp/core"
	"tgp/internal/parser"
)

// AstgPlugin реализует интерфейс Plugin.
type AstgPlugin struct{}

// Info возвращает информацию о плагине.
func (p *AstgPlugin) Info() core.PluginInfo {
	return core.PluginInfo{
		Name:        "astg",
		Persistent:  false,
		Version:     "1.0.0",
		Description: "AST transformer plugin that analyzes project and adds project data",
		Author:      "AlexK <seniorGolang@gmail.com>",
		License:     "MIT",
		Category:    "transformer",
	}
}

// Execute выполняет основную логику плагина.
func (p *AstgPlugin) Execute(rootDir string, request core.Storage, path ...string) (response core.Storage, err error) {

	// Создаем структурированный slog.Logger из core.Logger и устанавливаем как дефолтный
	coreLogger := core.GetLogger()
	slog.SetDefault(slog.New(newLoggerAdapter(coreLogger)).With(
		slog.String("plugin", "astg"),
	))

	slog.Info("astg transformer plugin started")

	// Создаем Response и копируем все данные из request
	response = core.NewStorage()
	if request != nil {
		if storageMap, ok := request.(*core.MapStorage); ok {
			for k, v := range *storageMap {
				if err = response.Set(k, v); err != nil {
					return nil, fmt.Errorf("failed to copy request data: %w", err)
				}
			}
		}
	}

	// Если project уже есть в request, не пересоздаем его
	if request != nil && request.Has("project") {
		slog.Debug("project already exists in request, skipping analysis")
		slog.Info("astg transformer plugin completed")
		return response, nil
	}

	// Получаем contracts из request или используем значение по умолчанию
	contractsDir := "contracts"
	if contractsVal, ok := request.Get("contracts"); ok {
		if contractsStr, ok := contractsVal.(string); ok && contractsStr != "" {
			contractsDir = contractsStr
		}
	}

	// Получаем список интерфейсов для фильтрации
	var ifaces []string
	if ifacesVal, ok := request.Get("ifaces"); ok {
		if ifacesStr, ok := ifacesVal.(string); ok && ifacesStr != "" {
			parts := strings.FieldsFunc(ifacesStr, func(r rune) bool {
				return r == ',' || r == ' ' || r == '\t'
			})
			ifaces = make([]string, 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					ifaces = append(ifaces, part)
				}
			}
		}
	}

	// В WASM файловая система монтируется в корень "/", поэтому используем относительные пути
	// rootDir уже является корнем файловой системы внутри WASM
	// contractsDir должен быть относительным путем от rootDir
	if filepath.IsAbs(contractsDir) {
		// Если передан абсолютный путь, вычисляем относительный путь от rootDir
		relPath, err := filepath.Rel(rootDir, contractsDir)
		if err != nil {
			return nil, fmt.Errorf("failed to compute relative path from rootDir: %w", err)
		}
		contractsDir = relPath
	}

	// Анализируем проект через parser.Collect
	// version - версия плагина astg, используется в сгенерированном коде как VersionTg
	pluginInfo := p.Info()
	slog.Info("analyzing project",
		slog.String("contractsDir", contractsDir),
		slog.Any("ifaces", ifaces),
		slog.String("version", pluginInfo.Version),
	)
	project, err := parser.Collect(slog.Default(), pluginInfo.Version, contractsDir, ifaces...)
	if err != nil {
		return nil, fmt.Errorf("failed to collect project: %w", err)
	}

	slog.Info("project analyzed",
		slog.String("modulePath", project.ModulePath),
		slog.Int("contractsCount", len(project.Contracts)),
	)

	// Добавляем project в response
	if err = response.Set("project", project); err != nil {
		return nil, fmt.Errorf("failed to set project in response: %w", err)
	}

	slog.Info("astg transformer plugin completed")
	return response, nil
}

// loggerAdapter адаптирует core.Logger к slog.Handler.
type loggerAdapter struct {
	logger core.Logger
}

// newLoggerAdapter создает новый адаптер.
func newLoggerAdapter(logger core.Logger) *loggerAdapter {
	return &loggerAdapter{logger: logger}
}

// Enabled проверяет, включен ли указанный уровень логирования.
func (a *loggerAdapter) Enabled(ctx context.Context, level slog.Level) bool {
	// Всегда включен, так как core.Logger не поддерживает уровни
	return true
}

// Handle обрабатывает запись лога.
func (a *loggerAdapter) Handle(ctx context.Context, record slog.Record) error {
	msg := record.Message
	// Добавляем атрибуты к сообщению, если они есть
	record.Attrs(func(attr slog.Attr) bool {
		msg += fmt.Sprintf(" %s=%v", attr.Key, attr.Value.Any())
		return true
	})

	switch record.Level {
	case slog.LevelError:
		a.logger.Error(msg)
	case slog.LevelWarn:
		a.logger.Warn(msg)
	case slog.LevelInfo:
		a.logger.Info(msg)
	case slog.LevelDebug:
		a.logger.Debug(msg)
	}
	return nil
}

// WithAttrs возвращает новый Handler с добавленными атрибутами.
func (a *loggerAdapter) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Простая реализация - возвращаем тот же адаптер
	return a
}

// WithGroup возвращает новый Handler с группой атрибутов.
func (a *loggerAdapter) WithGroup(name string) slog.Handler {
	// Простая реализация - возвращаем тот же адаптер
	return a
}

// pluginInstance - экземпляр плагина для регистрации.
var pluginInstance core.Plugin = &AstgPlugin{}
