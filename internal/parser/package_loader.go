// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log/slog"
	"path/filepath"
	"sync"

	"golang.org/x/tools/go/packages"
)

// PackageInfo содержит информацию о пакете.
type PackageInfo struct {
	PkgPath     string
	PackageName string // Имя пакета (package declaration), например "jose" для "github.com/go-jose/go-jose/v4"
	Files       []*ast.File
	MergedFile  *ast.File
	TypeInfo    *types.Info
	Fset        *token.FileSet
	Types       *types.Package
	Imports     map[string]string
}

type packageCache struct {
	mu    sync.RWMutex
	cache map[string]*PackageInfo
}

var globalPackageCache = &packageCache{
	cache: make(map[string]*PackageInfo),
}

// loadAllPackages загружает все пакеты проекта одним вызовом.
func loadAllPackages(log *slog.Logger, projectRoot string) error {
	log.Info("Loading all project packages...")

	cfg := &packages.Config{
		Mode:  packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps,
		Fset:  token.NewFileSet(),
		Dir:   projectRoot,
		Tests: false,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return fmt.Errorf("failed to load project packages: %w", err)
	}

	log.Info("Loading standard library packages...")
	stdPkgs, stdErr := packages.Load(cfg, "std")
	if stdErr != nil {
		log.Warn(fmt.Sprintf("Failed to load standard library: %v", stdErr))
	} else {
		pkgs = append(pkgs, stdPkgs...)
	}

	globalPackageCache.mu.Lock()
	defer globalPackageCache.mu.Unlock()

	seen := make(map[*packages.Package]bool)
	var visit func(*packages.Package)

	visit = func(pkg *packages.Package) {
		if seen[pkg] {
			return
		}
		seen[pkg] = true

		for path := range pkg.Imports {
			if imp, ok := pkg.Imports[path]; ok {
				visit(imp)
			}
		}

		pkgPath := pkg.PkgPath
		if pkgPath == "" {
			pkgPath = pkg.ID
		}

		if _, exists := globalPackageCache.cache[pkgPath]; exists {
			return
		}

		if pkg.Types == nil {
			return
		}

		info := &PackageInfo{
			PkgPath:     pkgPath,
			PackageName: pkg.Types.Name(),
			Files:       pkg.Syntax,
			Fset:        cfg.Fset,
			Types:       pkg.Types,
			Imports:     make(map[string]string),
		}

		if len(pkg.Syntax) > 0 {
			//nolint:staticcheck // ast.Package deprecated, but ast.MergePackageFiles still requires it
			astPkg := &ast.Package{
				Name:  pkg.Syntax[0].Name.Name,
				Files: make(map[string]*ast.File),
			}
			for i, file := range pkg.Syntax {
				var fileName string
				if i < len(pkg.GoFiles) {
					fileName = filepath.Base(pkg.GoFiles[i])
				}
				if fileName == "" && i < len(pkg.CompiledGoFiles) {
					fileName = pkg.CompiledGoFiles[i]
				}
				if fileName == "" {
					fileName = fmt.Sprintf("file%d.go", i)
				}
				astPkg.Files[fileName] = file
			}
			info.MergedFile = ast.MergePackageFiles(astPkg, ast.FilterUnassociatedComments|ast.FilterFuncDuplicates|ast.FilterImportDuplicates)
		}

		info.TypeInfo = pkg.TypesInfo

		if pkg.Types != nil {
			for _, importedPkg := range pkg.Types.Imports() {
				importPath := importedPkg.Path()
				packageName := importedPkg.Name()
				info.Imports[packageName] = importPath
			}
		}

		if info.MergedFile != nil {
			for _, imp := range info.MergedFile.Imports {
				if imp.Path == nil || len(imp.Path.Value) < 2 {
					continue
				}
				importPath := imp.Path.Value[1 : len(imp.Path.Value)-1]
				if imp.Name != nil {
					alias := imp.Name.Name
					info.Imports[alias] = importPath
				}
			}
		}

		globalPackageCache.cache[pkgPath] = info
	}

	for _, pkg := range pkgs {
		visit(pkg)
	}

	log.Info(fmt.Sprintf("Loaded %d packages", len(globalPackageCache.cache)))
	return nil
}

// getPackageInfo получает информацию о пакете из кэша.
// Если пакет не найден в кэше, пытается загрузить его динамически.
func getPackageInfo(log *slog.Logger, pkgPath string) (*PackageInfo, error) {
	globalPackageCache.mu.RLock()
	if info, ok := globalPackageCache.cache[pkgPath]; ok {
		globalPackageCache.mu.RUnlock()
		return info, nil
	}
	globalPackageCache.mu.RUnlock()

	// Пытаемся загрузить пакет динамически
	log.Debug(fmt.Sprintf("Package %s not found in cache, trying to load dynamically", pkgPath))

	globalPackageCache.mu.Lock()
	defer globalPackageCache.mu.Unlock()

	// Проверяем еще раз после блокировки на запись
	if info, ok := globalPackageCache.cache[pkgPath]; ok {
		return info, nil
	}

	// Загружаем пакет динамически
	cfg := &packages.Config{
		Mode:  packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps,
		Fset:  token.NewFileSet(),
		Tests: false,
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load package %s: %w", pkgPath, err)
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("package %s not found", pkgPath)
	}

	pkg := pkgs[0]
	if pkg.Types == nil {
		return nil, fmt.Errorf("package %s has no types", pkgPath)
	}

	// Сохраняем пакет в кэш
	info := &PackageInfo{
		PkgPath:     pkgPath,
		PackageName: pkg.Types.Name(),
		Files:       pkg.Syntax,
		Fset:        cfg.Fset,
		Types:       pkg.Types,
		Imports:     make(map[string]string),
	}

	if len(pkg.Syntax) > 0 {
		//nolint:staticcheck // ast.Package deprecated, but ast.MergePackageFiles still requires it
		astPkg := &ast.Package{
			Name:  pkg.Syntax[0].Name.Name,
			Files: make(map[string]*ast.File),
		}
		for i, file := range pkg.Syntax {
			var fileName string
			if i < len(pkg.GoFiles) {
				fileName = filepath.Base(pkg.GoFiles[i])
			}
			if fileName == "" && i < len(pkg.CompiledGoFiles) {
				fileName = pkg.CompiledGoFiles[i]
			}
			if fileName == "" {
				fileName = fmt.Sprintf("file%d.go", i)
			}
			astPkg.Files[fileName] = file
		}
		info.MergedFile = ast.MergePackageFiles(astPkg, ast.FilterUnassociatedComments|ast.FilterFuncDuplicates|ast.FilterImportDuplicates)
	}

	info.TypeInfo = pkg.TypesInfo

	if pkg.Types != nil {
		for _, importedPkg := range pkg.Types.Imports() {
			importPath := importedPkg.Path()
			packageName := importedPkg.Name()
			info.Imports[packageName] = importPath
		}
	}

	if info.MergedFile != nil {
		for _, imp := range info.MergedFile.Imports {
			if imp.Path == nil || len(imp.Path.Value) < 2 {
				continue
			}
			importPath := imp.Path.Value[1 : len(imp.Path.Value)-1]
			if imp.Name != nil {
				alias := imp.Name.Name
				info.Imports[alias] = importPath
			}
		}
	}

	globalPackageCache.cache[pkgPath] = info
	log.Debug(fmt.Sprintf("Dynamically loaded package %s", pkgPath))

	return info, nil
}
