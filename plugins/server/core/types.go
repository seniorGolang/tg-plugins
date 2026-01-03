// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package core

import (
	"tgp/plugins/server/tags"
)

// Project содержит всю собранную информацию о проекте.
type Project struct {
	Version      string `json:"version"`
	ModulePath   string `json:"modulePath"`
	ContractsDir string `json:"contractsDir"`

	Git *GitInfo `json:"git,omitempty"`

	Annotations tags.DocTags `json:"annotations,omitempty"`

	Services  []*Service       `json:"services,omitempty"`
	Contracts []*Contract      `json:"contracts,omitempty"`
	Types     map[string]*Type `json:"types,omitempty"`

	ExcludeDirs []string `json:"excludeDirs,omitempty"`
}

// GitInfo содержит информацию о Git репозитории.
type GitInfo struct {
	Commit    string `json:"commit"`
	Branch    string `json:"branch"`
	Tag       string `json:"tag,omitempty"`
	Dirty     bool   `json:"dirty"`
	User      string `json:"user,omitempty"`
	Email     string `json:"email,omitempty"`
	RemoteURL string `json:"remoteUrl,omitempty"`
}

// Service представляет группу контрактов, объединенных в одном исполняемом блоке.
type Service struct {
	Name        string   `json:"name"`
	MainPath    string   `json:"mainPath"`
	ContractIDs []string `json:"contractIds,omitempty"`
}

// Contract представляет Go интерфейс с аннотациями @tg.
type Contract struct {
	Name            string                `json:"name"`
	PkgPath         string                `json:"pkgPath"`
	FilePath        string                `json:"filePath"`
	ID              string                `json:"id"`
	Docs            []string              `json:"docs,omitempty"`
	Annotations     tags.DocTags          `json:"annotations,omitempty"`
	Methods         []*Method             `json:"methods,omitempty"`
	Implementations []*ImplementationInfo `json:"implementations,omitempty"`
}

// Method представляет метод контракта.
type Method struct {
	Name        string       `json:"name"`
	ContractID  string       `json:"contractID"`
	Args        []*Variable  `json:"args,omitempty"`
	Results     []*Variable  `json:"results,omitempty"`
	Docs        []string     `json:"docs,omitempty"`
	Annotations tags.DocTags `json:"annotations,omitempty"`
	Errors      []*ErrorInfo `json:"errors,omitempty"`
	Handler     *HandlerInfo `json:"handler,omitempty"`
}

// Variable представляет переменную (аргумент или результат метода).
type Variable struct {
	Name             string       `json:"name"`
	TypeID           string       `json:"typeID,omitempty"`
	NumberOfPointers int          `json:"numberOfPointers,omitempty"`
	IsSlice          bool         `json:"isSlice,omitempty"`
	ArrayLen         int          `json:"arrayLen,omitempty"`
	IsEllipsis       bool         `json:"isEllipsis,omitempty"`
	ElementPointers  int          `json:"elementPointers,omitempty"` // Для элементов массивов/слайсов и значений map
	MapKeyID         string       `json:"mapKeyID,omitempty"`
	MapValueID       string       `json:"mapValueID,omitempty"`
	MapKeyPointers   int          `json:"mapKeyPointers,omitempty"`
	Docs             []string     `json:"docs,omitempty"`
	Annotations      tags.DocTags `json:"annotations,omitempty"`
}

// HandlerInfo представляет информацию о кастомном обработчике.
type HandlerInfo struct {
	PkgPath string `json:"pkgPath"`
	Name    string `json:"name"`
}

// ImplementationInfo представляет информацию об имплементации контракта.
type ImplementationInfo struct {
	PkgPath    string                           `json:"pkgPath"`
	StructName string                           `json:"structName"`
	MethodsMap map[string]*ImplementationMethod `json:"methods,omitempty"`
}

// ImplementationMethod представляет метод имплементации контракта.
type ImplementationMethod struct {
	Name       string                `json:"name,omitempty"`
	FilePath   string                `json:"filePath"`
	ErrorTypes []*ErrorTypeReference `json:"errorTypes,omitempty"`
}

// ErrorInfo представляет информацию об ошибке метода.
type ErrorInfo struct {
	PkgPath      string `json:"pkgPath"`
	TypeName     string `json:"typeName"`
	FullName     string `json:"fullName"`
	HTTPCode     int    `json:"httpCode,omitempty"`
	HTTPCodeText string `json:"httpCodeText,omitempty"`
	TypeID       string `json:"typeID,omitempty"`
}

// ErrorTypeReference представляет ссылку на тип ошибки.
type ErrorTypeReference struct {
	PkgPath  string `json:"pkgPath"`
	TypeName string `json:"typeName"`
	FullName string `json:"fullName"`
}

// TypeKind представляет вид типа Go.
type TypeKind string

const (
	TypeKindString  TypeKind = "string"
	TypeKindInt     TypeKind = "int"
	TypeKindInt8    TypeKind = "int8"
	TypeKindInt16   TypeKind = "int16"
	TypeKindInt32   TypeKind = "int32"
	TypeKindInt64   TypeKind = "int64"
	TypeKindUint    TypeKind = "uint"
	TypeKindUint8   TypeKind = "uint8"
	TypeKindUint16  TypeKind = "uint16"
	TypeKindUint32  TypeKind = "uint32"
	TypeKindUint64  TypeKind = "uint64"
	TypeKindFloat32 TypeKind = "float32"
	TypeKindFloat64 TypeKind = "float64"
	TypeKindBool    TypeKind = "bool"
	TypeKindByte    TypeKind = "byte"
	TypeKindRune    TypeKind = "rune"
	TypeKindError   TypeKind = "error"
	TypeKindAny     TypeKind = "any"

	TypeKindArray     TypeKind = "array"
	TypeKindMap       TypeKind = "map"
	TypeKindChan      TypeKind = "chan"
	TypeKindStruct    TypeKind = "struct"
	TypeKindInterface TypeKind = "interface"
	TypeKindFunction  TypeKind = "function"
	TypeKindAlias     TypeKind = "alias"
)

// Type представляет сериализуемое представление типа Go.
type Type struct {
	Kind TypeKind `json:"kind,omitempty"`

	TypeName      string `json:"typeName,omitempty"`
	ImportAlias   string `json:"importAlias,omitempty"`
	ImportPkgPath string `json:"importPkgPath,omitempty"`
	PkgName       string `json:"pkgName,omitempty"` // Реальное имя пакета из package декларации

	AliasOf string `json:"aliasOf,omitempty"`

	ArrayLen        int    `json:"arrayLen,omitempty"`
	IsSlice         bool   `json:"isSlice,omitempty"`
	IsEllipsis      bool   `json:"isEllipsis,omitempty"`
	ArrayOfID       string `json:"arrayOfID,omitempty"`
	ElementPointers int    `json:"elementPointers,omitempty"` // Для элементов массивов/слайсов и значений map

	MapKeyID       string `json:"mapKeyID,omitempty"`
	MapValueID     string `json:"mapValueID,omitempty"`
	MapKeyPointers int    `json:"mapKeyPointers,omitempty"`

	ChanDirection int    `json:"chanDirection,omitempty"`
	ChanOfID      string `json:"chanOfID,omitempty"`

	StructFields []*StructField `json:"structFields,omitempty"`

	InterfaceMethods   []*Function `json:"interfaceMethods,omitempty"`
	EmbeddedInterfaces []*Variable `json:"embeddedInterfaces,omitempty"`

	FunctionArgs    []*Variable `json:"functionArgs,omitempty"`
	FunctionResults []*Variable `json:"functionResults,omitempty"`

	UnderlyingTypeID string   `json:"underlyingTypeID,omitempty"`
	UnderlyingKind   TypeKind `json:"underlyingKind,omitempty"`

	ImplementsInterfaces []string `json:"implementsInterfaces,omitempty"`
}

// StructField представляет поле структуры.
type StructField struct {
	Name             string              `json:"name"`
	TypeID           string              `json:"typeID,omitempty"`
	NumberOfPointers int                 `json:"numberOfPointers,omitempty"`
	IsSlice          bool                `json:"isSlice,omitempty"`
	ArrayLen         int                 `json:"arrayLen,omitempty"`
	IsEllipsis       bool                `json:"isEllipsis,omitempty"`
	ElementPointers  int                 `json:"elementPointers,omitempty"` // Для элементов массивов/слайсов и значений map
	MapKeyID         string              `json:"mapKeyID,omitempty"`
	MapValueID       string              `json:"mapValueID,omitempty"`
	MapKeyPointers   int                 `json:"mapKeyPointers,omitempty"`
	Tags             map[string][]string `json:"tags,omitempty"`
	Docs             []string            `json:"docs,omitempty"`
}

// Function представляет функцию или метод.
type Function struct {
	Name    string      `json:"name"`
	Args    []*Variable `json:"args,omitempty"`
	Results []*Variable `json:"results,omitempty"`
	Docs    []string    `json:"docs,omitempty"`
}
