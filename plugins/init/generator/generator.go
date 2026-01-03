// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package generator

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"tgp/shared"
)

//go:embed templates
var templates embed.FS

//go:embed pkg
var pkgFiles embed.FS

// GenerateSkeleton создает базовую структуру проекта.
// baseDir - относительный путь от rootDir (в WASM rootDir монтируется в корень файловой системы).
func GenerateSkeleton(moduleName, projectName, serviceName, baseDir string) (err error) {

	logger := shared.GetLogger()

	// В WASM файловая система монтируется в корень "/", поэтому используем относительные пути
	// baseDir уже является относительным путем от rootDir
	// Создаем базовую директорию
	if err = os.MkdirAll(baseDir, 0777); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	// Подготавливаем метаданные для шаблонов
	meta := map[string]string{
		"moduleName":       moduleName,
		"projectName":      projectName,
		"serviceNameCamel": ToCamel(serviceName),
		"projectNameCamel": ToLowerCamel(projectName),
		"serviceName":      ToLowerCamel(serviceName),
	}

	// Инициализируем go модуль
	logger.Info(fmt.Sprintf("initializing go module: %s", moduleName))
	_, err = shared.ExecuteCommandInDir("go", []string{"mod", "init", moduleName}, baseDir)
	if err != nil {
		return fmt.Errorf("failed to initialize go module: %w", err)
	}

	// Парсим шаблоны
	var tmpl *template.Template
	if tmpl, err = template.ParseFS(templates, "templates/*.tmpl"); err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	// Рендерим файлы
	if err = renderFile(tmpl, "main.tmpl", filepath.Join(baseDir, "cmd", serviceName, "main.go"), meta); err != nil {
		return fmt.Errorf("failed to render main.go: %w", err)
	}

	if err = renderFile(tmpl, "config.tmpl", filepath.Join(baseDir, "internal", "config", "service.go"), meta); err != nil {
		return fmt.Errorf("failed to render service.go: %w", err)
	}

	if err = renderFile(tmpl, "headers.tmpl", filepath.Join(baseDir, "internal", "utils", "header", "headers.go"), meta); err != nil {
		return fmt.Errorf("failed to render headers.go: %w", err)
	}

	if err = renderFile(tmpl, "golangci-lint.tmpl", filepath.Join(baseDir, ".golangci.yml"), meta); err != nil {
		return fmt.Errorf("failed to render .golangci.yml: %w", err)
	}

	if err = renderFile(tmpl, "ignore.tmpl", filepath.Join(baseDir, ".gitignore"), meta); err != nil {
		return fmt.Errorf("failed to render .gitignore: %w", err)
	}

	if err = renderFile(tmpl, "interface.tmpl", filepath.Join(baseDir, "contracts", fmt.Sprintf("%s.go", ToLowerCamel(serviceName))), meta); err != nil {
		return fmt.Errorf("failed to render contracts: %w", err)
	}

	// Копируем пакет dto
	if err = pkgCopyTo("dto", filepath.Join(baseDir, "contracts")); err != nil {
		return fmt.Errorf("failed to copy dto package: %w", err)
	}

	if err = renderFile(tmpl, "tg.tmpl", filepath.Join(baseDir, "contracts", "tg.go"), meta); err != nil {
		return fmt.Errorf("failed to render tg.go: %w", err)
	}

	if err = os.MkdirAll(filepath.Join(baseDir, "contracts", "dto"), 0777); err != nil {
		return fmt.Errorf("failed to create contracts/dto directory: %w", err)
	}

	if err = renderFile(tmpl, "service.tmpl", filepath.Join(baseDir, "internal", "services", ToLowerCamel(serviceName), "service.go"), meta); err != nil {
		return fmt.Errorf("failed to render service.go: %w", err)
	}

	if err = renderFile(tmpl, "service_method.tmpl", filepath.Join(baseDir, "internal", "services", ToLowerCamel(serviceName), "some.go"), meta); err != nil {
		return fmt.Errorf("failed to render some.go: %w", err)
	}

	// Копируем пакет errors
	if err = pkgCopyTo("errors", filepath.Join(baseDir, "pkg")); err != nil {
		return fmt.Errorf("failed to copy errors package: %w", err)
	}

	// Выполняем go generate в contracts
	logger.Info("running go generate in contracts")
	contractsDir := filepath.Join(baseDir, "contracts")
	// Вычисляем относительный путь от rootDir (который является базой для WASM)
	// В WASM rootDir монтируется в корень, поэтому используем относительные пути
	_, err = shared.ExecuteCommandInDir("go", []string{"generate"}, contractsDir)
	if err != nil {
		return fmt.Errorf("failed to run go generate: %w", err)
	}

	// Выполняем go mod tidy
	logger.Info("running go mod tidy")
	_, err = shared.ExecuteCommandInDir("go", []string{"mod", "tidy"}, baseDir)
	if err != nil {
		return fmt.Errorf("failed to run go mod tidy: %w", err)
	}

	return nil
}

// renderFile рендерит шаблон и записывает в файл.
func renderFile(tmpl *template.Template, templateName, filePath string, data any) (err error) {

	_ = os.Remove(filePath)
	dir := filepath.Dir(filePath)
	if err = os.MkdirAll(dir, 0777); err != nil {
		return
	}
	var buf bytes.Buffer
	if err = tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		return
	}
	return os.WriteFile(filePath, buf.Bytes(), 0600)
}

// pkgCopyTo копирует пакет из embed FS в указанную директорию.
func pkgCopyTo(pkg, dst string) (err error) {

	pkgPath := path.Join("pkg", pkg)
	entries, err := pkgFiles.ReadDir(pkgPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var fileContent []byte
		filePath := path.Join(pkgPath, entry.Name())
		if fileContent, err = pkgFiles.ReadFile(filePath); err != nil {
			return err
		}
		if err = os.MkdirAll(path.Join(dst, pkg), 0700); err != nil {
			return err
		}
		filename := filepath.Join(dst, pkg, entry.Name())
		if err = os.WriteFile(filename, fileContent, 0600); err != nil {
			return err
		}
	}
	return
}
