package main

import (
	"tgp/shared"
)

// ClientTsPlugin реализует интерфейс Plugin.
type ClientTsPlugin struct{}

// Info возвращает информацию о плагине.
func (p *ClientTsPlugin) Info() shared.PluginInfo {
	return shared.PluginInfo{
		Name:        "client TypeScript",
		Persistent:  false,
		Version:     "2.4.0",
		Description: "Плагин client-ts",
		Author:      "Rick Sanchez <rick.sanchez@example.com>",
		License:     "MIT",
		Category:    "client",
		Commands: []shared.Command{
			{
				Path:        []string{"client", "ts"},
				Description: "Плагин client-ts",
				Options:     []shared.Option{},
			},
		},
		Options: []shared.Option{},
	}
}

// Execute выполняет основную логику плагина.
func (p *ClientTsPlugin) Execute(project shared.Project, rootDir string, options map[string]any, path ...string) (err error) {
	logger := shared.GetLogger()

	logger.Info("clientTs плагин запущен")

	// TODO: Реализовать основную логику плагина
	// Используйте стандартные Go функции для работы с файлами через WASI:
	// - os.ReadFile(), os.WriteFile(), os.Open() и т.д.

	logger.Info("clientTs плагин завершён")
	return
}

// pluginInstance - экземпляр плагина для регистрации.
var pluginInstance shared.Plugin = &ClientTsPlugin{}
