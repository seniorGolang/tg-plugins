// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package tsg

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// File представляет TypeScript файл для генерации (аналог jen.File)
type File struct {
	imports     map[string]importInfo
	statements  []*Statement
	comment     string
	indentLevel int
}

type importInfo struct {
	path      string
	alias     string
	named     []string
	namedType []string // type-only импорты
	defaulted string
}

// NewFile создаёт новый TypeScript файл
func NewFile() *File {
	return &File{
		imports:     make(map[string]importInfo),
		statements:  make([]*Statement, 0),
		indentLevel: 0,
	}
}

// Comment устанавливает комментарий в начале файла
func (f *File) Comment(comment string) *File {
	f.comment = comment
	return f
}

// Import добавляет импорт по умолчанию
func (f *File) Import(path string, defaultName string) *File {
	info := f.imports[path]
	info.path = path
	info.defaulted = defaultName
	f.imports[path] = info
	return f
}

// ImportNamed добавляет именованный импорт
func (f *File) ImportNamed(path string, names ...string) *File {
	info := f.imports[path]
	info.path = path
	info.named = append(info.named, names...)
	f.imports[path] = info
	return f
}

// ImportType добавляет type-only именованный импорт
func (f *File) ImportType(path string, names ...string) *File {
	info := f.imports[path]
	info.path = path
	info.namedType = append(info.namedType, names...)
	f.imports[path] = info
	return f
}

// ImportAll добавляет импорт всех экспортов
func (f *File) ImportAll(path string, alias string) *File {
	info := f.imports[path]
	info.path = path
	info.alias = alias
	f.imports[path] = info
	return f
}

// GenerateImports генерирует импорты как statements и добавляет их в начало statements
func (f *File) GenerateImports() *File {
	if len(f.imports) == 0 {
		return f
	}

	paths := make([]string, 0, len(f.imports))
	for path := range f.imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	importStatements := make([]*Statement, 0, len(paths))
	for _, path := range paths {
		info := f.imports[path]
		
		// Обычные импорты (default, named, type) - отдельно от namespace import
		var parts []string
		if info.defaulted != "" {
			parts = append(parts, info.defaulted)
		}

		// Объединяем обычные и type-only импорты в одну строку
		var namedImports []string
		if len(info.named) > 0 {
			namedImports = append(namedImports, info.named...)
		}
		if len(info.namedType) > 0 {
			for _, name := range info.namedType {
				namedImports = append(namedImports, "type "+name)
			}
		}
		if len(namedImports) > 0 {
			parts = append(parts, "{"+strings.Join(namedImports, ", ")+"}")
		}

		if len(parts) > 0 {
			stmt := NewStatement()
			stmt.Import(strings.Join(parts, ", "), path)
			stmt.Line()
			importStatements = append(importStatements, stmt)
		}
		
		// Если есть alias (namespace import), создаем отдельный импорт
		if info.alias != "" {
			stmt := NewStatement()
			stmt.ImportAll(info.alias, path)
			stmt.Line()
			importStatements = append(importStatements, stmt)
		}
	}

	// Добавляем импорты в начало statements
	f.statements = append(importStatements, f.statements...)
	return f
}

// Add добавляет statement в файл
func (f *File) Add(stmt *Statement) *File {
	if stmt != nil {
		f.statements = append(f.statements, stmt)
	}
	return f
}

// Line добавляет пустую строку
func (f *File) Line() *File {
	f.statements = append(f.statements, NewStatement().Line())
	return f
}

// Save сохраняет файл
func (f *File) Save(filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0777); err != nil {
		return err
	}
	return os.WriteFile(filename, []byte(f.String()), 0600)
}

// String возвращает строковое представление файла
func (f *File) String() string {
	var buf strings.Builder

	// Комментарий - пишем как есть, без модификаций
	if f.comment != "" {
		buf.WriteString(f.comment)
	}

	// Statements - пишем как есть, без модификаций
	for _, stmt := range f.statements {
		buf.WriteString(stmt.String())
	}

	return buf.String()
}
