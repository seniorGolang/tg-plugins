package main

import (
	"tgp/shared"
)

// ClientGoPlugin реализует интерфейс Plugin.
type ClientGoPlugin struct{}

// Info возвращает информацию о плагине.
func (p *ClientGoPlugin) Info() shared.PluginInfo {
	return shared.PluginInfo{
		Name:        "client Golang",
		Persistent:  false,
		Version:     "2.4.0",
		Description: "Плагин client-go",
		Author:      "Rick Sanchez <rick.sanchez@example.com>",
		License:     "MIT",
		Category:    "client",
		Commands: []shared.Command{
			{
				Path:        []string{"client", "go"},
				Description: "Плагин client-go",
				Options:     []shared.Option{},
			},
		},
		Options: []shared.Option{},
	}
}

// Execute выполняет основную логику плагина.
func (p *ClientGoPlugin) Execute(project shared.Project, rootDir string, options map[string]any, path ...string) (err error) {
	logger := shared.GetLogger()

	logger.Info("clientGo плагин запущен")

	// TODO: Реализовать основную логику плагина
	// Используйте стандартные Go функции для работы с файлами через WASI:
	// - os.ReadFile(), os.WriteFile(), os.Open() и т.д.

	logger.Info("clientGo плагин завершён")
	return
}

// pluginInstance - экземпляр плагина для регистрации.
var pluginInstance shared.Plugin = &ClientGoPlugin{}
