package core

import (
	"encoding/json"
	"errors"
)

// Project содержит всю собранную информацию о проекте.
type Project struct {
	Version      string `json:"version"`
	ModulePath   string `json:"modulePath"`
	ContractsDir string `json:"contractsDir"`

	Git *GitInfo `json:"git,omitempty"`

	Annotations map[string]string `json:"annotations,omitempty"` // tags.DocTags заменен на map[string]string для WASM

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
	Annotations     map[string]string    `json:"annotations,omitempty"` // tags.DocTags заменен на map[string]string для WASM
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
	Annotations map[string]string `json:"annotations,omitempty"` // tags.DocTags заменен на map[string]string для WASM
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
	Annotations      map[string]string `json:"annotations,omitempty"` // tags.DocTags заменен на map[string]string для WASM
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

	ArrayLen      int    `json:"arrayLen,omitempty"`
	IsSlice       bool   `json:"isSlice,omitempty"`
	IsEllipsis    bool   `json:"isEllipsis,omitempty"`
	ArrayOfID     string `json:"arrayOfID,omitempty"`
	ElementPointers int  `json:"elementPointers,omitempty"` // Для элементов массивов/слайсов и значений map

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

// PluginInfo содержит информацию о плагине.
// Все поля сериализуемы в JSON для совместимости с WASM.
type PluginInfo struct {
	// Name - уникальное имя плагина.
	Name string `json:"name"`

	Persistent bool `json:"persistent"`

	// Version - версия плагина (семантическое версионирование).
	Version string `json:"version"`

	// Description - краткое описание плагина.
	Description string `json:"description"`

	// Author - автор плагина.
	Author string `json:"author"`

	// License - лицензия плагина.
	License string `json:"license"`

	// Category - категория плагина (объявляется плагином, например: "server", "client", "documentation").
	Category string `json:"category"`

	// Commands - список команд, которые предоставляет плагин.
	// Каждая команда может иметь свой путь, описание и опции.
	Commands []Command `json:"commands"`

	// Group - группа плагинов для совместимости (плагины из одной группы совместимы друг с другом).
	Group string `json:"group,omitempty"`

	// Doc - документация плагина в формате Markdown.
	Doc string `json:"doc"`

	// Options - описание настроек, которые плагин принимает (общие для всех команд, если не переопределены в Command).
	Options []Option `json:"options"`

	// AllowedHTTP - белый список адресов для HTTP запросов (только для WASM плагинов).
	// Если пусто, плагин не может выполнять HTTP запросы.
	// Формат: домены, пути, regex шаблоны (например: ["example.com", "api.example.com", "*.example.com", "^https://api\\.example\\.com/.*"])
	AllowedHTTP []string `json:"allowedHTTP,omitempty"`

	// AllowedShellCMDs - белый список команд, которые плагин может запускать через хост (только для WASM плагинов).
	// Если пусто, плагин не может выполнять команды.
	// Формат: имена команд (например: ["go", "git", "npm"])
	AllowedShellCMDs []string `json:"allowedShellCMDs,omitempty"`

	// Dependencies - список зависимостей плагина.
	// Формат: ["plugin-name1@^1.0.0", "plugin-name2@^2.1.0"]
	// Версия указывается с учетом semver совместимости (^, ~, >= и т.д.).
	// Для трансформеров определяет порядок выполнения.
	// Для команд определяет плагины, которые должны выполниться перед командой.
	// Если версия не указана, то совместимость с любыми версиями.
	// Если зависимость не установлена, tg предлагает её установить.
	Dependencies []string `json:"dependencies,omitempty"`

	// AlwaysRun - для трансформеров: выполнять ли всегда перед командами.
	// По умолчанию false (выполняется только при наличии зависимостей).
	AlwaysRun bool `json:"alwaysRun,omitempty"`
}

// PluginType определяет тип плагина.
type PluginType string

const (
	// PluginTypeCommand - плагин-команда.
	PluginTypeCommand PluginType = "command"
	// PluginTypeTransformer - плагин-трансформер.
	PluginTypeTransformer PluginType = "transformer"
)

type Command struct {
	Path            []string `json:"path"`
	Description     string   `json:"description"`
	LongDescription string   `json:"longDescription,omitempty"` // Подробное описание команды (для help). Если не указано, используется info.Doc
	// Options - описание настроек, которые плагин принимает.
	Options []Option `json:"options"`
}

// Option описывает одну настройку плагина.
// Все поля сериализуемы в JSON для совместимости с WASM.
type Option struct {
	// Name - полное имя настройки (ключ, используется как --name в CLI).
	Name string `json:"name"`

	// Short - краткое имя настройки (используется как -s в CLI, опционально).
	Short string `json:"short,omitempty"`

	// Type - тип значения настройки (string, int, bool и т.д.).
	Type string `json:"type"`

	// Description - описание настройки.
	Description string `json:"description"`

	// Required - обязательна ли настройка.
	Required bool `json:"required"`

	// Default - значение по умолчанию (может быть nil).
	// Должно быть JSON-сериализуемым для совместимости с WASM.
	Default any `json:"default,omitempty"`

	// IsPositional - true для позиционных аргументов (не флагов).
	// Позиционные аргументы передаются без флагов, например: `tg plugin install <package>`.
	IsPositional bool `json:"isPositional,omitempty"`
}

// Storage определяет интерфейс хранилища для запросов и ответов плагинов.
// Request и Response должны быть одного типа, реализующего интерфейс Storage.
type Storage interface {
	// Get возвращает значение поля по имени.
	Get(name string) (value any, ok bool)

	// Set устанавливает значение поля по имени.
	Set(name string, value any) (err error)

	// Has проверяет наличие поля по имени.
	Has(name string) (has bool)

	// MarshalJSON сериализует хранилище в JSON.
	MarshalJSON() (data []byte, err error)

	// UnmarshalJSON десериализует хранилище из JSON.
	UnmarshalJSON(data []byte) (err error)
}

// MapStorage - универсальная реализация Storage на основе map[string]any.
type MapStorage map[string]any

// NewStorage создает новый экземпляр Storage.
func NewStorage() Storage {
	s := make(MapStorage)
	return &s
}

// Get возвращает значение поля по имени.
func (s MapStorage) Get(name string) (value any, ok bool) {
	if s == nil {
		return nil, false
	}
	value, ok = s[name]
	return
}

// Set устанавливает значение поля по имени.
func (s MapStorage) Set(name string, value any) (err error) {
	if s == nil {
		return errors.New("storage is nil")
	}
	s[name] = value
	return nil
}

// Has проверяет наличие поля по имени.
func (s MapStorage) Has(name string) (has bool) {
	if s == nil {
		return false
	}
	_, has = s[name]
	return
}

// MarshalJSON сериализует хранилище в JSON.
func (s MapStorage) MarshalJSON() (data []byte, err error) {
	if s == nil {
		return json.Marshal(map[string]any{})
	}
	return json.Marshal(map[string]any(s))
}

// UnmarshalJSON десериализует хранилище из JSON.
func (s *MapStorage) UnmarshalJSON(data []byte) (err error) {
	if s == nil {
		return errors.New("storage is nil")
	}
	var m map[string]any
	if err = json.Unmarshal(data, &m); err != nil {
		return err
	}
	*s = MapStorage(m)
	return nil
}

// Plugin определяет интерфейс плагина.
type Plugin interface {
	// Info возвращает информацию о плагине.
	Info() PluginInfo

	// Execute выполняет основную логику плагина.
	// rootDir - абсолютный путь к корневой директории файловой системы, доступной плагину для чтения и записи.
	// Для обычных плагинов это обычно корень проекта (директория с go.mod).
	// Для WASM плагинов эта директория монтируется в корень файловой системы WASI (/).
	// request - запрос плагина, реализующий интерфейс Storage.
	// Request - универсальный Storage (MapStorage), содержащий данные от предыдущего шага.
	// Планировщик проверяет `Has("project")` для автоматического добавления зависимости от project-loader.
	// response - ответ плагина, реализующий интерфейс Storage.
	// Request и Response должны быть одного типа, что обеспечивает полную совместимость.
	// path - путь команды, которая была вызвана (например, ["plugin", "init"]).
	Execute(rootDir string, request Storage, path ...string) (response Storage, err error)
}

// ExecuteRequest содержит аргументы для Execute.
type ExecuteRequest struct {
	RootDir string   `json:"rootDir"`
	Request Storage  `json:"request"` // Storage напрямую
	Path    []string `json:"path,omitempty"` // путь команды, которая была вызвана
}

// UnmarshalJSON десериализует ExecuteRequest из JSON.
// Автоматически создает MapStorage для Request.
func (req *ExecuteRequest) UnmarshalJSON(data []byte) (err error) {
	var aux struct {
		RootDir string          `json:"rootDir"`
		Request json.RawMessage `json:"request"`
		Path    []string        `json:"path,omitempty"`
	}
	if err = json.Unmarshal(data, &aux); err != nil {
		return err
	}
	req.RootDir = aux.RootDir
	req.Path = aux.Path
	if len(aux.Request) > 0 {
		storage := NewStorage()
		if err = json.Unmarshal(aux.Request, storage); err != nil {
			return err
		}
		req.Request = storage
	} else {
		req.Request = NewStorage()
	}
	return nil
}

// ExecuteResponse содержит результат выполнения Execute.
type ExecuteResponse struct {
	Error    string  `json:"error,omitempty"`
	Response Storage `json:"response,omitempty"` // Storage напрямую
}

// CommandResponse представляет результат выполнения команды через хост.
type CommandResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}
