package shared

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
	Annotations     map[string]string     `json:"annotations,omitempty"` // tags.DocTags заменен на map[string]string для WASM
	Methods         []*Method             `json:"methods,omitempty"`
	Implementations []*ImplementationInfo `json:"implementations,omitempty"`
}

// Method представляет метод контракта.
type Method struct {
	Name        string            `json:"name"`
	ContractID  string            `json:"contractID"`
	Args        []*Variable       `json:"args,omitempty"`
	Results     []*Variable       `json:"results,omitempty"`
	Docs        []string          `json:"docs,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"` // tags.DocTags заменен на map[string]string для WASM
	Errors      []*ErrorInfo      `json:"errors,omitempty"`
	Handler     *HandlerInfo      `json:"handler,omitempty"`
}

// Variable представляет переменную (аргумент или результат метода).
type Variable struct {
	Name             string            `json:"name"`
	TypeID           string            `json:"typeID,omitempty"`
	NumberOfPointers int               `json:"numberOfPointers,omitempty"`
	IsSlice          bool              `json:"isSlice,omitempty"`
	ArrayLen         int               `json:"arrayLen,omitempty"`
	IsEllipsis       bool              `json:"isEllipsis,omitempty"`
	ElementPointers  int               `json:"elementPointers,omitempty"` // Для элементов массивов/слайсов и значений map
	MapKeyID         string            `json:"mapKeyID,omitempty"`
	MapValueID       string            `json:"mapValueID,omitempty"`
	MapKeyPointers   int               `json:"mapKeyPointers,omitempty"`
	Docs             []string          `json:"docs,omitempty"`
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

	// NoProject - указывает, что плагину не нужны данные проекта Go (контракты, сервисы, типы).
	// Если true, парсинг проекта через core.Collect не выполняется, и в Execute передается пустой проект.
	// По умолчанию false (проект нужен) для обратной совместимости.
	NoProject bool `json:"noProject,omitempty"`
}

type Command struct {
	Path        []string `json:"path"`
	Description string   `json:"description"`
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
}

// Plugin определяет интерфейс плагина.
type Plugin interface {
	// Info возвращает информацию о плагине.
	Info() PluginInfo

	// Execute выполняет основную логику плагина.
	// project - данные проекта (передаются по значению для WASM совместимости).
	// rootDir - абсолютный путь к корневой директории файловой системы, доступной плагину для чтения и записи.
	// Для обычных плагинов это обычно корень проекта (директория с go.mod).
	// Для WASM плагинов эта директория монтируется в корень файловой системы WASI (/).
	// options - опции выполнения плагина.
	// path - путь команды, которая была вызвана (например, ["plugin", "init"]).
	Execute(project Project, rootDir string, options map[string]any, path ...string) (err error)
}

// ExecuteRequest содержит аргументы для Execute.
type ExecuteRequest struct {
	Project interface{}    `json:"project"`
	RootDir string         `json:"rootDir"`
	Options map[string]any `json:"options"`
	Path    []string       `json:"path,omitempty"` // путь команды, которая была вызвана
}

// ExecuteResponse содержит результат выполнения Execute.
type ExecuteResponse struct {
	Error string `json:"error,omitempty"`
}

// CommandResponse представляет результат выполнения команды через хост.
type CommandResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}
