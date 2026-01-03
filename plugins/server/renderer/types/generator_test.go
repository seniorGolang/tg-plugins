// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package types

import (
	"testing"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/plugins/server/core"
)

// mockSrcFile реализует интерфейс SrcFile для тестирования.
type mockSrcFile struct {
	imports map[string]string
}

func newMockSrcFile() *mockSrcFile {
	return &mockSrcFile{
		imports: make(map[string]string),
	}
}

func (m *mockSrcFile) ImportName(pkgPath, name string) {
	m.imports[pkgPath] = name
}

func (m *mockSrcFile) ImportAlias(pkgPath, alias string) {
	m.imports[pkgPath] = alias
}

func (m *mockSrcFile) Add(code ...*Statement) *Statement {
	return &Statement{}
}

func TestGenerator_FieldType_Caching(t *testing.T) {

	project := &core.Project{
		Types: map[string]*core.Type{
			"string": {
				Kind: core.TypeKindString,
			},
			"int": {
				Kind: core.TypeKindInt,
			},
		},
	}

	srcFile := newMockSrcFile()
	gen := NewGenerator(project, srcFile)

	// Первый вызов - должен сгенерировать и закэшировать
	result1 := gen.FieldType("string", 0, false)
	if result1 == nil {
		t.Fatal("FieldType returned nil")
	}

	// Второй вызов с теми же параметрами - должен вернуть из кэша
	result2 := gen.FieldType("string", 0, false)
	if result1 != result2 {
		t.Error("FieldType should return cached result for same parameters")
	}

	// Проверяем, что кэш работает
	if len(gen.typeCache) == 0 {
		t.Error("typeCache should not be empty after FieldType call")
	}

	// Разные параметры - должны быть разные ключи кэша
	result3 := gen.FieldType("string", 1, false) // с указателем
	if result1 == result3 {
		t.Error("FieldType should return different result for different parameters")
	}

	// Проверяем, что в кэше два элемента
	if len(gen.typeCache) != 2 {
		t.Errorf("Expected 2 cache entries, got %d", len(gen.typeCache))
	}
}

func TestGenerator_FieldType_PrimitiveTypes(t *testing.T) {

	project := &core.Project{
		Types: map[string]*core.Type{
			"string": {
				Kind: core.TypeKindString,
			},
			"int": {
				Kind: core.TypeKindInt,
			},
			"bool": {
				Kind: core.TypeKindBool,
			},
		},
	}

	srcFile := newMockSrcFile()
	gen := NewGenerator(project, srcFile)

	tests := []struct {
		name             string
		typeID           string
		numberOfPointers int
		expectedResult   bool
	}{
		{"string type", "string", 0, true},
		{"int type", "int", 0, true},
		{"bool type", "bool", 0, true},
		{"string pointer", "string", 1, true},
		{"unknown type", "unknown", 0, true}, // Должен вернуть результат, но с другим типом
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.FieldType(tt.typeID, tt.numberOfPointers, false)
			if tt.expectedResult && result == nil {
				t.Errorf("FieldType(%q, %d, false) returned nil", tt.typeID, tt.numberOfPointers)
			}
		})
	}
}

func TestGenerator_FieldType_ArrayTypes(t *testing.T) {

	project := &core.Project{
		Types: map[string]*core.Type{
			"string": {
				Kind: core.TypeKindString,
			},
			"[]string": {
				Kind:      core.TypeKindArray,
				IsSlice:   true,
				ArrayOfID: "string",
			},
		},
	}

	srcFile := newMockSrcFile()
	gen := NewGenerator(project, srcFile)

	result := gen.FieldType("[]string", 0, false)
	if result == nil {
		t.Fatal("FieldType for array returned nil")
	}
}

func TestGenerator_FieldTypeFromVariable(t *testing.T) {

	project := &core.Project{
		Types: map[string]*core.Type{
			"string": {
				Kind: core.TypeKindString,
			},
		},
	}

	srcFile := newMockSrcFile()
	gen := NewGenerator(project, srcFile)

	tests := []struct {
		name     string
		variable *core.Variable
	}{
		{
			name: "simple variable",
			variable: &core.Variable{
				TypeID:           "string",
				NumberOfPointers: 0,
			},
		},
		{
			name: "slice variable",
			variable: &core.Variable{
				TypeID:           "string",
				NumberOfPointers: 0,
				IsSlice:          true,
			},
		},
		{
			name: "pointer variable",
			variable: &core.Variable{
				TypeID:           "string",
				NumberOfPointers: 1,
			},
		},
		{
			name: "ellipsis variable",
			variable: &core.Variable{
				TypeID:           "string",
				NumberOfPointers: 0,
				IsEllipsis:       true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.FieldTypeFromVariable(tt.variable, true)
			if result == nil {
				t.Errorf("FieldTypeFromVariable returned nil for %s", tt.name)
			}
		})
	}
}

func TestGenerator_FuncDefinitionParams(t *testing.T) {

	project := &core.Project{
		Types: map[string]*core.Type{
			"string": {
				Kind: core.TypeKindString,
			},
			"int": {
				Kind: core.TypeKindInt,
			},
		},
	}

	srcFile := newMockSrcFile()
	gen := NewGenerator(project, srcFile)

	vars := []*core.Variable{
		{
			Name:   "name",
			TypeID: "string",
		},
		{
			Name:   "age",
			TypeID: "int",
		},
	}

	result := gen.FuncDefinitionParams(vars)
	if result == nil {
		t.Fatal("FuncDefinitionParams returned nil")
	}
}
