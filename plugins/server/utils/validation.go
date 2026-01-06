// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package utils

import (
	"fmt"
	"strings"

	"tgp/internal/parser"
)

// ValidateProject проверяет корректность проекта.
func ValidateProject(project *parser.Project) error {

	if project == nil {
		return fmt.Errorf("project cannot be nil")
	}
	if project.ModulePath == "" {
		return fmt.Errorf("project.ModulePath cannot be empty")
	}
	return nil
}

// ValidateContractID проверяет корректность ID контракта.
func ValidateContractID(contractID string) error {

	if contractID == "" {
		return fmt.Errorf("contractID cannot be empty")
	}
	return nil
}

// ValidateOutDir проверяет корректность выходной директории.
func ValidateOutDir(outDir string) error {

	if outDir == "" {
		return fmt.Errorf("outDir cannot be empty")
	}
	return nil
}

// FindContract находит контракт по ID и возвращает ошибку, если не найден.
func FindContract(project *parser.Project, contractID string) (*parser.Contract, error) {

	if project == nil {
		return nil, fmt.Errorf("project cannot be nil")
	}
	if contractID == "" {
		return nil, fmt.Errorf("contractID cannot be empty")
	}

	for _, contract := range project.Contracts {
		if contract.ID == contractID {
			return contract, nil
		}
	}

	return nil, fmt.Errorf("contract %q not found", contractID)
}

// ValidateContract проверяет корректность контракта.
func ValidateContract(contract *parser.Contract, project *parser.Project) error {

	if contract == nil {
		return fmt.Errorf("contract cannot be nil")
	}

	for _, method := range contract.Methods {
		// Проверяем именование параметров (кроме context.Context)
		for i, arg := range method.Args {
			if arg.Name == "" && arg.TypeID != "context:Context" {
				return fmt.Errorf("contract %q: method %q: argument #%d has no name (all arguments except context.Context must be named)", contract.Name, method.Name, i+1)
			}
		}

		// Проверяем именование возвращаемых значений (кроме error)
		for i, result := range method.Results {
			if result.Name == "" && result.TypeID != "error" {
				return fmt.Errorf("contract %q: method %q: result #%d has no name (all results except error must be named)", contract.Name, method.Name, i+1)
			}
		}

		// Проверяем аргументы на наличие неподдерживаемых типов (рекурсивно)
		for _, arg := range method.Args {
			if err := validateVariable(arg, project, contract.Name, method.Name, "argument"); err != nil {
				return err
			}
		}

		// Проверяем результаты на наличие неподдерживаемых типов (рекурсивно)
		for _, result := range method.Results {
			if err := validateVariable(result, project, contract.Name, method.Name, "result"); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateVariable проверяет переменную и все вложенные типы на наличие неподдерживаемых типов.
func validateVariable(v *parser.Variable, project *parser.Project, contractName, methodName, varType string) error {

	// Проверяем на дженерики (типы с параметрами типа)
	if err := validateVariableForGenerics(v, project, contractName, methodName, varType); err != nil {
		return err
	}

	// Проверяем базовые неподдерживаемые типы
	if err := validateVariableForChan(v, project, contractName, methodName, varType); err != nil {
		return err
	}
	if err := validateVariableForFunction(v, project, contractName, methodName, varType); err != nil {
		return err
	}
	if err := validateVariableForUnsafe(v, project, contractName, methodName, varType); err != nil {
		return err
	}
	if err := validateVariableForInterface(v, project, contractName, methodName, varType); err != nil {
		return err
	}

	// Рекурсивно проверяем поля структур
	typ, ok := project.Types[v.TypeID]
	if ok && typ.Kind == parser.TypeKindStruct {
		for _, field := range typ.StructFields {
			fieldVar := &parser.Variable{
				Name:             field.Name,
				TypeID:           field.TypeID,
				NumberOfPointers: field.NumberOfPointers,
				IsSlice:          field.IsSlice,
				ArrayLen:         field.ArrayLen,
				IsEllipsis:       field.IsEllipsis,
				ElementPointers:  field.ElementPointers,
				MapKeyID:         field.MapKeyID,
				MapValueID:       field.MapValueID,
				MapKeyPointers:   field.MapKeyPointers,
			}
			if err := validateVariable(fieldVar, project, contractName, methodName, fmt.Sprintf("%s.%s", varType, field.Name)); err != nil {
				return err
			}
		}
	}

	// Проверяем алиасы структур
	if ok && typ.Kind == parser.TypeKindAlias && typ.AliasOf != "" {
		aliasType, ok := project.Types[typ.AliasOf]
		if ok && aliasType.Kind == parser.TypeKindStruct {
			for _, field := range aliasType.StructFields {
				fieldVar := &parser.Variable{
					Name:             field.Name,
					TypeID:           field.TypeID,
					NumberOfPointers: field.NumberOfPointers,
					IsSlice:          field.IsSlice,
					ArrayLen:         field.ArrayLen,
					IsEllipsis:       field.IsEllipsis,
					ElementPointers:  field.ElementPointers,
					MapKeyID:         field.MapKeyID,
					MapValueID:       field.MapValueID,
					MapKeyPointers:   field.MapKeyPointers,
				}
				if err := validateVariable(fieldVar, project, contractName, methodName, fmt.Sprintf("%s.%s", varType, field.Name)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateVariableForGenerics проверяет переменную на наличие дженериков (не поддерживаются).
func validateVariableForGenerics(v *parser.Variable, project *parser.Project, contractName, methodName, varType string) error {

	// Проверяем TypeID на наличие параметров типа (дженерики)
	// Дженерики в Go имеют формат типа "package:Type[T]" или содержат квадратные скобки
	if strings.Contains(v.TypeID, "[") && strings.Contains(v.TypeID, "]") {
		return fmt.Errorf("contract %q: method %q: %s %q has unsupported generic type %q (generics are not supported)", contractName, methodName, varType, v.Name, v.TypeID)
	}

	// Проверяем map значения на наличие дженериков
	if v.MapValueID != "" {
		if strings.Contains(v.MapValueID, "[") && strings.Contains(v.MapValueID, "]") {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported generic type in map value %q (generics are not supported)", contractName, methodName, varType, v.Name, v.MapValueID)
		}
	}

	// Проверяем map ключи на наличие дженериков
	if v.MapKeyID != "" {
		if strings.Contains(v.MapKeyID, "[") && strings.Contains(v.MapKeyID, "]") {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported generic type in map key %q (generics are not supported)", contractName, methodName, varType, v.Name, v.MapKeyID)
		}
	}

	// Проверяем элементы слайса на наличие дженериков
	if v.IsSlice {
		if strings.Contains(v.TypeID, "[") && strings.Contains(v.TypeID, "]") {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported generic type in slice element %q (generics are not supported)", contractName, methodName, varType, v.Name, v.TypeID)
		}
	}

	// Проверяем тип в project.Types на наличие информации о дженериках
	// Если TypeID содержит параметры типа, это дженерик
	// (проверка уже выполнена выше, но оставляем для полноты)

	return nil
}

// validateVariableForChan проверяет переменную на наличие chan типа.
func validateVariableForChan(v *parser.Variable, project *parser.Project, contractName, methodName, varType string) error {

	// Проверяем, является ли тип каналом
	typ, ok := project.Types[v.TypeID]
	if ok && typ.Kind == parser.TypeKindChan {
		return fmt.Errorf("contract %q: method %q: %s %q has unsupported type chan (channels are not supported)", contractName, methodName, varType, v.Name)
	}

	// Проверяем map значения на наличие chan
	if v.MapValueID != "" {
		mapValueType, ok := project.Types[v.MapValueID]
		if ok && mapValueType.Kind == parser.TypeKindChan {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with chan value (channels are not supported)", contractName, methodName, varType, v.Name)
		}
	}

	// Проверяем map ключи на наличие chan
	if v.MapKeyID != "" {
		mapKeyType, ok := project.Types[v.MapKeyID]
		if ok && mapKeyType.Kind == parser.TypeKindChan {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with chan key (channels are not supported)", contractName, methodName, varType, v.Name)
		}
	}

	// Проверяем элементы слайса на наличие chan
	if v.IsSlice {
		// Для слайсов TypeID содержит тип элемента
		elementType, ok := project.Types[v.TypeID]
		if ok && elementType.Kind == parser.TypeKindChan {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type slice of chan (channels are not supported)", contractName, methodName, varType, v.Name)
		}
	}

	return nil
}

// validateVariableForFunction проверяет переменную на наличие function типа.
func validateVariableForFunction(v *parser.Variable, project *parser.Project, contractName, methodName, varType string) error {

	// Проверяем, является ли тип функцией
	typ, ok := project.Types[v.TypeID]
	if ok && typ.Kind == parser.TypeKindFunction {
		return fmt.Errorf("contract %q: method %q: %s %q has unsupported type func (function types are not supported)", contractName, methodName, varType, v.Name)
	}

	// Проверяем map значения на наличие function
	if v.MapValueID != "" {
		mapValueType, ok := project.Types[v.MapValueID]
		if ok && mapValueType.Kind == parser.TypeKindFunction {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with func value (function types are not supported)", contractName, methodName, varType, v.Name)
		}
	}

	// Проверяем map ключи на наличие function
	if v.MapKeyID != "" {
		mapKeyType, ok := project.Types[v.MapKeyID]
		if ok && mapKeyType.Kind == parser.TypeKindFunction {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with func key (function types are not supported)", contractName, methodName, varType, v.Name)
		}
	}

	// Проверяем элементы слайса на наличие function
	if v.IsSlice {
		// Для слайсов TypeID содержит тип элемента
		elementType, ok := project.Types[v.TypeID]
		if ok && elementType.Kind == parser.TypeKindFunction {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type slice of func (function types are not supported)", contractName, methodName, varType, v.Name)
		}
	}

	return nil
}

// validateVariableForUnsafe проверяет переменную на наличие unsafe типов.
func validateVariableForUnsafe(v *parser.Variable, project *parser.Project, contractName, methodName, varType string) error {

	// Проверяем, является ли тип unsafe.Pointer
	if v.TypeID == "unsafe:Pointer" {
		return fmt.Errorf("contract %q: method %q: %s %q has unsupported type unsafe.Pointer (unsafe types are not supported)", contractName, methodName, varType, v.Name)
	}

	// Проверяем map значения на наличие unsafe
	if v.MapValueID == "unsafe:Pointer" {
		return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with unsafe.Pointer value (unsafe types are not supported)", contractName, methodName, varType, v.Name)
	}

	// Проверяем map ключи на наличие unsafe
	if v.MapKeyID == "unsafe:Pointer" {
		return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with unsafe.Pointer key (unsafe types are not supported)", contractName, methodName, varType, v.Name)
	}

	// Проверяем элементы слайса на наличие unsafe
	if v.IsSlice && v.TypeID == "unsafe:Pointer" {
		return fmt.Errorf("contract %q: method %q: %s %q has unsupported type slice of unsafe.Pointer (unsafe types are not supported)", contractName, methodName, varType, v.Name)
	}

	return nil
}

// validateVariableForInterface проверяет переменную на наличие interface{} типа (не поддерживается, кроме any и context.Context).
func validateVariableForInterface(v *parser.Variable, project *parser.Project, contractName, methodName, varType string) error {

	// Проверяем, является ли тип interface{} (но any и context.Context поддерживаются)
	typ, ok := project.Types[v.TypeID]
	if ok && typ.Kind == parser.TypeKindInterface {
		// context.Context - это интерфейс, но он поддерживается
		if v.TypeID == "context:Context" {
			return nil
		}
		// Проверяем, является ли это именованным интерфейсом или пустым interface{}
		// Пустой interface{} имеет TypeID вида "package:interface:anonymous" или просто "interface{}"
		if strings.Contains(v.TypeID, ":interface:anonymous") || v.TypeID == "interface{}" {
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type interface{} (use 'any' instead)", contractName, methodName, varType, v.Name)
		}
		// Именованные интерфейсы тоже не поддерживаются
		return fmt.Errorf("contract %q: method %q: %s %q has unsupported type interface (interfaces are not supported)", contractName, methodName, varType, v.Name)
	}

	// Проверяем map значения на наличие interface
	if v.MapValueID != "" {
		mapValueType, ok := project.Types[v.MapValueID]
		if ok && mapValueType.Kind == parser.TypeKindInterface {
			if strings.Contains(v.MapValueID, ":interface:anonymous") || v.MapValueID == "interface{}" {
				return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with interface{} value (use 'any' instead)", contractName, methodName, varType, v.Name)
			}
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with interface value (interfaces are not supported)", contractName, methodName, varType, v.Name)
		}
	}

	// Проверяем map ключи на наличие interface
	if v.MapKeyID != "" {
		mapKeyType, ok := project.Types[v.MapKeyID]
		if ok && mapKeyType.Kind == parser.TypeKindInterface {
			if strings.Contains(v.MapKeyID, ":interface:anonymous") || v.MapKeyID == "interface{}" {
				return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with interface{} key (use 'any' instead)", contractName, methodName, varType, v.Name)
			}
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type map with interface key (interfaces are not supported)", contractName, methodName, varType, v.Name)
		}
	}

	// Проверяем элементы слайса на наличие interface
	if v.IsSlice {
		elementType, ok := project.Types[v.TypeID]
		if ok && elementType.Kind == parser.TypeKindInterface {
			if strings.Contains(v.TypeID, ":interface:anonymous") || v.TypeID == "interface{}" {
				return fmt.Errorf("contract %q: method %q: %s %q has unsupported type slice of interface{} (use 'any' instead)", contractName, methodName, varType, v.Name)
			}
			return fmt.Errorf("contract %q: method %q: %s %q has unsupported type slice of interface (interfaces are not supported)", contractName, methodName, varType, v.Name)
		}
	}

	return nil
}
