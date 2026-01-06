// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	"tgp/internal/common"
	"tgp/internal/mod"
	"tgp/internal/tags"
)

// Collect собирает всю информацию о проекте из AST.
func Collect(log *slog.Logger, version, svcDir string, ifaces ...string) (*Project, error) {
	return CollectWithExcludeDirs(log, version, svcDir, nil, ifaces...)
}

// CollectWithExcludeDirs собирает всю информацию о проекте из AST с указанием исключаемых директорий.
func CollectWithExcludeDirs(log *slog.Logger, version, svcDir string, excludeDirs []string, ifaces ...string) (*Project, error) {
	project := &Project{
		Version:      version,
		ContractsDir: svcDir,
		Types:        make(map[string]*Type),
		Contracts:    make([]*Contract, 0),
		Services:     make([]*Service, 0),
		ExcludeDirs:  excludeDirs,
	}

	// Получаем информацию о модуле и корень проекта
	modPath, err := mod.GoModPath(svcDir)
	if err != nil {
		log.Debug("Failed to get go.mod path", "svcDir", svcDir, "error", err)
		return nil, fmt.Errorf("failed to get go.mod path: %w", err)
	}

	log.Debug("Found go.mod", "modPath", modPath, "svcDir", svcDir)

	projectRoot, err := filepath.Abs(filepath.Dir(modPath))
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute project root path: %w", err)
	}

	modBytes, err := os.ReadFile(modPath)
	switch {
	case err != nil:
		log.Debug("Failed to read go.mod, using svcDir as ModulePath", "modPath", modPath, "error", err, "svcDir", svcDir)
		project.ModulePath = svcDir
	default:
		modFile, err := modfile.Parse(modPath, modBytes, nil)
		switch {
		case err != nil:
			log.Debug("Failed to parse go.mod, using svcDir as ModulePath", "modPath", modPath, "error", err, "svcDir", svcDir)
			project.ModulePath = svcDir
		case modFile.Module != nil:
			project.ModulePath = modFile.Module.Mod.Path
			log.Debug("Parsed ModulePath from go.mod", "modulePath", project.ModulePath, "modPath", modPath)
		default:
			log.Debug("go.mod has no module, using svcDir as ModulePath", "modPath", modPath, "svcDir", svcDir)
			project.ModulePath = svcDir
		}
	}

	// Собираем Git информацию
	if err := collectGitInfo(project); err != nil {
		log.Debug("Failed to collect git info", "error", err)
	}

	// Загружаем все пакеты проекта сразу
	if err := loadAllPackages(log, projectRoot); err != nil {
		return nil, fmt.Errorf("failed to load all project packages: %w", err)
	}

	// Парсим интерфейсы
	// svcDir должен быть абсолютным путем (core.Collect вызывается из обычного окружения, не WASM)
	files, err := os.ReadDir(svcDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read service directory: %w", err)
	}

	include := make([]string, 0, len(ifaces))
	exclude := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		if strings.HasPrefix(iface, "!") {
			exclude = append(exclude, strings.TrimPrefix(iface, "!"))
			continue
		}
		include = append(include, iface)
	}

	if len(include) != 0 && len(exclude) != 0 {
		return nil, fmt.Errorf("include and exclude cannot be set at same time (%v | %v)", include, exclude)
	}

	// Собираем контракты из файлов
	contractsMap := make(map[string]*Contract)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}

		svcDirAbs, _ := filepath.Abs(svcDir)
		filePathAbs := filepath.Join(svcDirAbs, file.Name())

		// Получаем путь пакета
		pkgPath, err := common.GetPkgPath(filepath.Dir(filePathAbs), true)
		if err != nil {
			log.Warn(fmt.Sprintf("Failed to get package path for %s: %v", filePathAbs, err))
			fset := token.NewFileSet()
			astFile, err := parseFile(fset, filePathAbs)
			if err != nil {
				return nil, fmt.Errorf("failed to parse file %s: %w", filePathAbs, err)
			}
			pkgPath = astFile.Name.Name
		}

		// Получаем информацию о пакете из кэша
		pkgInfo, err := getPackageInfo(log, pkgPath)
		if err != nil {
			log.Warn(fmt.Sprintf("Package %s not found in cache, skipping file %s: %v", pkgPath, filePathAbs, err))
			continue
		}

		// Парсим текущий файл
		fset := token.NewFileSet()
		astFile, err := parseFile(fset, filePathAbs)
		if err != nil {
			log.Warn(fmt.Sprintf("Failed to parse file %s: %v", filePathAbs, err))
			continue
		}

		// Собираем импорты из текущего файла
		imports := make(map[string]string)
		for _, imp := range astFile.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			var alias string
			if imp.Name != nil {
				alias = imp.Name.Name
			} else {
				parts := strings.Split(importPath, "/")
				if len(parts) > 0 {
					alias = parts[len(parts)-1]
				}
			}
			imports[alias] = importPath
		}

		// Собираем глобальные теги проекта из комментариев пакета
		if astFile.Doc != nil && len(project.Annotations) == 0 {
			packageDocs := extractComments(astFile.Doc)
			project.Annotations = project.Annotations.Merge(tags.ParseTags(packageDocs))
		}

		// Преобразуем абсолютный путь в относительный от корня проекта
		filePathRel := makeRelativePath(filePathAbs, projectRoot)

		// Обрабатываем интерфейсы из AST
		for _, decl := range astFile.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}

				interfaceName := typeSpec.Name.Name

				// Фильтрация по include/exclude
				if len(include) != 0 {
					if !contains(include, interfaceName) {
						continue
					}
				}
				if len(exclude) != 0 {
					if contains(exclude, interfaceName) {
						continue
					}
				}

				// Пропускаем интерфейсы без аннотаций @tg
				interfaceDocs := extractComments(genDecl.Doc, typeSpec.Doc, typeSpec.Comment)
				ifaceAnnotations := tags.ParseTags(interfaceDocs)
				if len(ifaceAnnotations) == 0 {
					continue
				}

				// Создаем контракт
				contractID := fmt.Sprintf("%s:%s", pkgPath, interfaceName)
				contract := &Contract{
					ID:          contractID,
					Name:        interfaceName,
					PkgPath:     pkgPath,
					FilePath:    filePathRel,
					Docs:        removeAnnotationsFromDocs(interfaceDocs),
					Annotations: ifaceAnnotations,
					Methods:     make([]*Method, 0),
				}

				// Обрабатываем методы интерфейса
				if interfaceType.Methods != nil {
					typeInfo := pkgInfo.TypeInfo
					for _, methodField := range interfaceType.Methods.List {
						// Пропускаем встроенные интерфейсы
						if _, ok := methodField.Type.(*ast.Ident); ok {
							continue
						}
						if _, ok := methodField.Type.(*ast.SelectorExpr); ok {
							continue
						}

						// Обрабатываем только методы (FuncType)
						funcType, ok := methodField.Type.(*ast.FuncType)
						if !ok {
							continue
						}

						// Имя метода
						methodName := ""
						if len(methodField.Names) > 0 && methodField.Names[0] != nil {
							methodName = methodField.Names[0].Name
						}
						if methodName == "" {
							continue
						}

						method := convertMethod(log, methodName, funcType, extractComments(methodField.Doc, methodField.Comment), contractID, pkgPath, imports, typeInfo, project)
						if method != nil {
							contract.Methods = append(contract.Methods, method)
						}
					}
				}

				contractsMap[contractID] = contract
			}
		}
	}

	// Преобразуем map в slice
	for _, contract := range contractsMap {
		project.Contracts = append(project.Contracts, contract)
	}

	// Выполняем полный анализ проекта
	if err := analyzeProject(log, project); err != nil {
		return nil, fmt.Errorf("failed to analyze project: %w", err)
	}

	return project, nil
}

// parseFile парсит Go файл.
func parseFile(fset *token.FileSet, filename string) (*ast.File, error) {
	return parser.ParseFile(fset, filename, nil, parser.ParseComments)
}

// extractComments извлекает комментарии из CommentGroup.
func extractComments(commentGroups ...*ast.CommentGroup) []string {
	var comments []string
	for _, group := range commentGroups {
		if group == nil {
			continue
		}
		for _, comment := range group.List {
			comments = append(comments, comment.Text)
		}
	}
	return comments
}

// makeRelativePath преобразует абсолютный путь в относительный от корня проекта.
func makeRelativePath(absPath, projectRoot string) string {
	relPath, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(relPath)
}

// removeAnnotationsFromDocs удаляет строки с аннотациями @tg из комментариев.
func removeAnnotationsFromDocs(docs []string) []string {
	if len(docs) == 0 {
		return docs
	}

	filtered := make([]string, 0, len(docs))
	for _, doc := range docs {
		trimmed := strings.TrimSpace(strings.TrimPrefix(doc, "//"))
		if strings.HasPrefix(trimmed, "@tg") {
			continue
		}
		filtered = append(filtered, doc)
	}
	return filtered
}

// contains проверяет, содержится ли строка в слайсе.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
