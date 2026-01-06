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

	"tgp/internal/common"
	"tgp/internal/mod"
)

// StructInfo содержит информацию о структуре из AST.
type StructInfo struct {
	Name   string
	Fields []*ast.Field
	Doc    *ast.CommentGroup
}

// findImplementations находит все имплементации контрактов в проекте.
func findImplementations(log *slog.Logger, project *Project) error {
	goProjectPath := mod.GoProjectPath(project.ContractsDir)
	if goProjectPath == "" {
		goProjectPath = mod.GoProjectPath(".")
		if goProjectPath == "" {
			log.Warn(fmt.Sprintf("Failed to find project root, using ContractsDir: %s", project.ContractsDir))
			goProjectPath = project.ContractsDir
		}
	}

	for _, contract := range project.Contracts {
		implementations := findContractImplementations(log, contract, goProjectPath, project)
		contract.Implementations = implementations
	}

	return nil
}

// findContractImplementations находит имплементации конкретного контракта.
func findContractImplementations(log *slog.Logger, contract *Contract, projectRoot string, project *Project) []*ImplementationInfo {
	implementations := make([]*ImplementationInfo, 0)

	packages := make(map[string][]string)
	seenImplementations := make(map[string]bool)

	err := filepath.Walk(projectRoot, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if info.Name() == "vendor" {
				return filepath.SkipDir
			}
			if shouldExcludeDir(filePath, projectRoot, project.ExcludeDirs) {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}

		if isGeneratedFile(filePath) {
			return nil
		}

		if shouldExcludeDir(filepath.Dir(filePath), projectRoot, project.ExcludeDirs) {
			return nil
		}

		pkgDir := filepath.Dir(filePath)
		pkgPath, err := common.GetPkgPath(pkgDir, true)
		if err != nil {
			log.Debug(fmt.Sprintf("Failed to get package path for %s: %v", filePath, err))
			return nil
		}

		pkgPath = filepath.ToSlash(pkgPath)

		if _, exists := packages[pkgPath]; !exists {
			packages[pkgPath] = make([]string, 0)
		}
		packages[pkgPath] = append(packages[pkgPath], filePath)

		return nil
	})

	if err != nil {
		log.With("error", err).Warn("Failed to walk project directory")
		return implementations
	}

	for pkgPath, goFiles := range packages {
		if len(goFiles) == 0 {
			continue
		}

		fset := token.NewFileSet()
		parsedFiles := make([]*ast.File, 0)
		for _, filePath := range goFiles {
			parsedFile, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
			if err != nil {
				log.Debug(fmt.Sprintf("Failed to parse file %s: %v", filePath, err))
				continue
			}
			parsedFiles = append(parsedFiles, parsedFile)
		}

		if len(parsedFiles) == 0 {
			continue
		}

		//nolint:staticcheck // ast.Package deprecated, but ast.MergePackageFiles still requires it
		astPkg := &ast.Package{
			Name:  parsedFiles[0].Name.Name,
			Files: make(map[string]*ast.File),
		}
		for i, file := range parsedFiles {
			astPkg.Files[fmt.Sprintf("file%d.go", i)] = file
		}
		mergedFile := ast.MergePackageFiles(astPkg, ast.FilterUnassociatedComments|ast.FilterFuncDuplicates|ast.FilterImportDuplicates)

		structs := findStructsInFile(mergedFile)
		for _, structType := range structs {
			implKey := fmt.Sprintf("%s:%s", pkgPath, structType.Name)
			if seenImplementations[implKey] {
				log.Debug(fmt.Sprintf("Implementation %s already processed, skipping", implKey))
				continue
			}

			if implementsContract(log, structType, contract, mergedFile, goFiles[0], pkgPath, project) {
				impl := &ImplementationInfo{
					PkgPath:    pkgPath,
					StructName: structType.Name,
					MethodsMap: make(map[string]*ImplementationMethod),
				}

				for _, method := range contract.Methods {
					var implMethod *ImplementationMethod
					for _, filePath := range goFiles {
						implMethod = findImplementationMethod(log, method, structType, mergedFile, filePath, pkgPath, projectRoot, project)
						if implMethod != nil {
							break
						}
					}
					if implMethod != nil {
						impl.MethodsMap[method.Name] = implMethod
					}
				}

				if len(impl.MethodsMap) > 0 {
					implementations = append(implementations, impl)
					seenImplementations[implKey] = true
				}
			}
		}
	}

	return implementations
}

// findStructsInFile находит все структуры в файле.
func findStructsInFile(file *ast.File) []StructInfo {
	var structs []StructInfo
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			structs = append(structs, StructInfo{
				Name:   typeSpec.Name.Name,
				Fields: structType.Fields.List,
				Doc:    genDecl.Doc,
			})
		}
	}
	return structs
}

// implementsContract проверяет, реализует ли структура контракт.
func implementsContract(log *slog.Logger, structType StructInfo, contract *Contract, file *ast.File, filePath, pkgPath string, project *Project) bool {
	for _, contractMethod := range contract.Methods {
		foundMethod := findMethodInFile(file, structType.Name, contractMethod.Name)

		if foundMethod == nil {
			log.Debug(fmt.Sprintf("Method %s not found in struct %s", contractMethod.Name, structType.Name))
			return false
		}

		goProjectPath := mod.GoProjectPath(project.ContractsDir)
		if goProjectPath == "" {
			goProjectPath = "."
		}
		contractFilePathAbs, err := filepath.Abs(filepath.Join(goProjectPath, contract.FilePath))
		if err != nil {
			log.Debug(fmt.Sprintf("Failed to get absolute path for contract file: %v", err))
			contractFilePathAbs = filepath.Join(goProjectPath, contract.FilePath)
		}
		if !methodSignaturesMatch(log, contractMethod, foundMethod, contractFilePathAbs, contract.PkgPath, pkgPath, project) {
			log.Debug(fmt.Sprintf("Method %s signature mismatch in struct %s", contractMethod.Name, structType.Name))
			return false
		}
	}

	return true
}

// findMethodInFile находит метод в файле по имени структуры и имени метода.
func findMethodInFile(file *ast.File, structName, methodName string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Name == nil || funcDecl.Name.Name != methodName {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		recvType := funcDecl.Recv.List[0].Type
		if isReceiverForStruct(recvType, structName) {
			return funcDecl
		}
	}
	return nil
}

// isReceiverForStruct проверяет, соответствует ли receiver типу структуры.
func isReceiverForStruct(recvType ast.Expr, structName string) bool {
	switch rt := recvType.(type) {
	case *ast.Ident:
		return rt.Name == structName
	case *ast.StarExpr:
		if ident, ok := rt.X.(*ast.Ident); ok {
			return ident.Name == structName
		}
	}
	return false
}

// findImplementationMethod находит метод имплементации для метода контракта.
func findImplementationMethod(log *slog.Logger, contractMethod *Method, structType StructInfo, file *ast.File, filePath, pkgPath, projectRoot string, project *Project) *ImplementationMethod {
	foundMethod := findMethodInFile(file, structType.Name, contractMethod.Name)

	if foundMethod == nil {
		return nil
	}

	var contract *Contract
	for _, c := range project.Contracts {
		if c.ID == contractMethod.ContractID {
			contract = c
			break
		}
	}
	if contract == nil {
		log.Debug(fmt.Sprintf("Contract %s not found", contractMethod.ContractID))
		return nil
	}

	goProjectPath := mod.GoProjectPath(project.ContractsDir)
	if goProjectPath == "" {
		goProjectPath = "."
	}
	contractFilePathAbs, err := filepath.Abs(filepath.Join(goProjectPath, contract.FilePath))
	if err != nil {
		contractFilePathAbs = filepath.Join(goProjectPath, contract.FilePath)
	}
	if !methodSignaturesMatch(log, contractMethod, foundMethod, contractFilePathAbs, contract.PkgPath, pkgPath, project) {
		log.Debug(fmt.Sprintf("Method %s signature mismatch, skipping", contractMethod.Name))
		return nil
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		log.Warn(fmt.Sprintf("Failed to parse file %s for method analysis: %v", filePath, err))
		filePathRel := makeRelativePath(filePath, projectRoot)
		return &ImplementationMethod{
			FilePath: filePathRel,
		}
	}

	var methodAST *ast.FuncDecl
	ast.Inspect(astFile, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Name != nil && fn.Name.Name == contractMethod.Name {
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					recvType := fn.Recv.List[0].Type
					if ident, ok := recvType.(*ast.Ident); ok && ident.Name == structType.Name {
						methodAST = fn
						return false
					}
					if star, ok := recvType.(*ast.StarExpr); ok {
						if ident, ok := star.X.(*ast.Ident); ok && ident.Name == structType.Name {
							methodAST = fn
							return false
						}
					}
				}
			}
		}
		return true
	})

	filePathRel := makeRelativePath(filePath, projectRoot)
	implMethod := &ImplementationMethod{
		FilePath: filePathRel,
	}

	if methodAST != nil && methodAST.Body != nil {
		errorTypes := findErrorTypesInMethodBody(log, methodAST.Body, astFile, pkgPath)
		implMethod.ErrorTypes = errorTypes
	}

	return implMethod
}

// methodSignaturesMatch проверяет, совпадают ли сигнатуры методов контракта и имплементации.
func methodSignaturesMatch(log *slog.Logger, contractMethod *Method, implMethod *ast.FuncDecl, contractFilePath, contractPkgPath, implPkgPath string, project *Project) bool {
	contractMethodType, err := getContractMethod(log, contractFilePath, contractPkgPath, contractMethod.Name, project)
	if err != nil {
		log.Debug(fmt.Sprintf("Failed to get contract method AST for %s: %v", contractMethod.Name, err))
		return false
	}

	return compareMethodSignatures(contractMethodType, implMethod, contractPkgPath, implPkgPath, project)
}

// getContractMethod получает AST тип метода контракта из исходного файла.
func getContractMethod(log *slog.Logger, contractFilePathAbs, contractPkgPath, methodName string, project *Project) (*ast.FuncType, error) {
	var contract *Contract
	goProjectPath := mod.GoProjectPath(project.ContractsDir)
	if goProjectPath == "" {
		goProjectPath = "."
	}
	for _, c := range project.Contracts {
		if c.PkgPath == contractPkgPath {
			cFilePathAbs, err := filepath.Abs(filepath.Join(goProjectPath, c.FilePath))
			if err != nil {
				cFilePathAbs = filepath.Join(goProjectPath, c.FilePath)
			}
			if cFilePathAbs == contractFilePathAbs {
				contract = c
				break
			}
		}
	}
	if contract == nil {
		return nil, fmt.Errorf("contract not found for pkgPath %s", contractPkgPath)
	}

	fset := token.NewFileSet()
	contractFile, err := parser.ParseFile(fset, contractFilePathAbs, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract file: %w", err)
	}

	for _, decl := range contractFile.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typeSpec.Name.Name != contract.Name {
				continue
			}
			interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			if interfaceType.Methods != nil {
				for _, methodField := range interfaceType.Methods.List {
					if _, ok := methodField.Type.(*ast.Ident); ok {
						continue
					}
					if _, ok := methodField.Type.(*ast.SelectorExpr); ok {
						continue
					}
					funcType, ok := methodField.Type.(*ast.FuncType)
					if !ok {
						continue
					}
					if len(methodField.Names) > 0 && methodField.Names[0].Name == methodName {
						return funcType, nil
					}
				}
			}
			return nil, fmt.Errorf("method %s not found in contract %s", methodName, contract.Name)
		}
	}

	return nil, fmt.Errorf("contract interface %s not found in file", contract.Name)
}

// compareMethodSignatures сравнивает сигнатуры методов напрямую на основе AST типов.
func compareMethodSignatures(contractMethod *ast.FuncType, implMethod *ast.FuncDecl, contractPkgPath, implPkgPath string, project *Project) bool {
	implFuncType := implMethod.Type

	contractParamsCount := 0
	if contractMethod.Params != nil {
		for _, param := range contractMethod.Params.List {
			if len(param.Names) > 0 {
				contractParamsCount += len(param.Names)
			} else {
				contractParamsCount++
			}
		}
	}
	implParamsCount := 0
	if implFuncType.Params != nil {
		for _, param := range implFuncType.Params.List {
			if len(param.Names) > 0 {
				implParamsCount += len(param.Names)
			} else {
				implParamsCount++
			}
		}
	}
	if contractParamsCount != implParamsCount {
		return false
	}

	contractResultsCount := 0
	if contractMethod.Results != nil {
		for _, result := range contractMethod.Results.List {
			if len(result.Names) > 0 {
				contractResultsCount += len(result.Names)
			} else {
				contractResultsCount++
			}
		}
	}
	implResultsCount := 0
	if implFuncType.Results != nil {
		for _, result := range implFuncType.Results.List {
			if len(result.Names) > 0 {
				implResultsCount += len(result.Names)
			} else {
				implResultsCount++
			}
		}
	}
	if contractResultsCount != implResultsCount {
		return false
	}

	contractParams := contractMethod.Params
	implParams := implFuncType.Params
	switch {
	case contractParams == nil && implParams == nil:
	case contractParams == nil || implParams == nil:
		return false
	default:
		if len(contractParams.List) != len(implParams.List) {
			return false
		}
		for i := range contractParams.List {
			if !compareTypesRecursive(contractParams.List[i].Type, implParams.List[i].Type, contractPkgPath, implPkgPath, project) {
				return false
			}
		}
	}

	contractResults := contractMethod.Results
	implResults := implFuncType.Results
	switch {
	case contractResults == nil && implResults == nil:
	case contractResults == nil || implResults == nil:
		return false
	default:
		if len(contractResults.List) != len(implResults.List) {
			return false
		}
		for i := range contractResults.List {
			if !compareTypesRecursive(contractResults.List[i].Type, implResults.List[i].Type, contractPkgPath, implPkgPath, project) {
				return false
			}
		}
	}

	return true
}

// compareTypesRecursive рекурсивно сравнивает AST типы.
func compareTypesRecursive(contractType, implType ast.Expr, contractPkgPath, implPkgPath string, project *Project) bool {
	if contractType == nil && implType == nil {
		return true
	}
	if contractType == nil || implType == nil {
		return false
	}

	contractBaseType := contractType
	implBaseType := implType
	contractPtrCount := 0
	implPtrCount := 0

	for {
		if star, ok := contractBaseType.(*ast.StarExpr); ok {
			contractPtrCount++
			contractBaseType = star.X
		} else {
			break
		}
	}

	for {
		if star, ok := implBaseType.(*ast.StarExpr); ok {
			implPtrCount++
			implBaseType = star.X
		} else {
			break
		}
	}

	if contractPtrCount != implPtrCount {
		return false
	}

	switch contractT := contractBaseType.(type) {
	case *ast.Ident:
		implT, ok := implBaseType.(*ast.Ident)
		if !ok {
			return false
		}
		return contractT.Name == implT.Name

	case *ast.ArrayType:
		implT, ok := implBaseType.(*ast.ArrayType)
		if !ok {
			return false
		}
		if contractT.Len != nil && implT.Len != nil {
			contractLit, contractOk := contractT.Len.(*ast.BasicLit)
			implLit, implOk := implT.Len.(*ast.BasicLit)
			if contractOk && implOk {
				if contractLit.Value != implLit.Value {
					return false
				}
			} else if contractT.Len != nil || implT.Len != nil {
				return false
			}
		} else if (contractT.Len != nil) != (implT.Len != nil) {
			return false
		}
		return compareTypesRecursive(contractT.Elt, implT.Elt, contractPkgPath, implPkgPath, project)

	case *ast.MapType:
		implT, ok := implBaseType.(*ast.MapType)
		if !ok {
			return false
		}
		if !compareTypesRecursive(contractT.Key, implT.Key, contractPkgPath, implPkgPath, project) {
			return false
		}
		return compareTypesRecursive(contractT.Value, implT.Value, contractPkgPath, implPkgPath, project)

	case *ast.SelectorExpr:
		implT, ok := implBaseType.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if contractT.Sel.Name != implT.Sel.Name {
			return false
		}
		contractX, contractOk := contractT.X.(*ast.Ident)
		implX, implOk := implT.X.(*ast.Ident)
		if !contractOk || !implOk {
			return false
		}
		return contractX.Name == implX.Name

	default:
		return false
	}
}

// findErrorTypesInMethodBody анализирует AST тела функции для поиска типов ошибок.
func findErrorTypesInMethodBody(log *slog.Logger, body *ast.BlockStmt, file *ast.File, pkgPath string) []*ErrorTypeReference {
	errorTypes := make([]*ErrorTypeReference, 0)
	errorTypesMap := make(map[string]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		if retStmt, ok := n.(*ast.ReturnStmt); ok {
			for _, result := range retStmt.Results {
				extractErrorTypeFromExpr(result, file, pkgPath, errorTypesMap, &errorTypes)
			}
			return true
		}

		if assignStmt, ok := n.(*ast.AssignStmt); ok {
			for i, lhs := range assignStmt.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "err" {
					if i < len(assignStmt.Rhs) {
						extractErrorTypeFromExpr(assignStmt.Rhs[i], file, pkgPath, errorTypesMap, &errorTypes)
					}
				}
			}
			return true
		}

		return true
	})

	return errorTypes
}

// extractErrorTypeFromExpr извлекает тип ошибки из выражения.
func extractErrorTypeFromExpr(expr ast.Expr, file *ast.File, pkgPath string, errorTypesMap map[string]bool, errorTypes *[]*ErrorTypeReference) {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		if selExpr, ok := e.Type.(*ast.SelectorExpr); ok {
			if x, ok := selExpr.X.(*ast.Ident); ok {
				pkgName := x.Name
				typeName := selExpr.Sel.Name

				for _, imp := range file.Imports {
					impPath := strings.Trim(imp.Path.Value, "\"")
					var impName string
					if imp.Name != nil {
						impName = imp.Name.Name
					} else {
						parts := strings.Split(impPath, "/")
						impName = parts[len(parts)-1]
					}

					if impName == pkgName {
						key := fmt.Sprintf("%s:%s", impPath, typeName)
						if !errorTypesMap[key] {
							errorTypesMap[key] = true
							*errorTypes = append(*errorTypes, &ErrorTypeReference{
								PkgPath:  impPath,
								TypeName: typeName,
								FullName: fmt.Sprintf("%s.%s", impPath, typeName),
							})
						}
						break
					}
				}
			}
		}
	case *ast.CallExpr:
		for _, arg := range e.Args {
			extractErrorTypeFromExpr(arg, file, pkgPath, errorTypesMap, errorTypes)
		}
	}
}
