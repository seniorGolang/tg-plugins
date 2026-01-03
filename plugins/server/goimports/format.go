package goimports

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	linebreak = '\n'
	indent    = '\t'
)

// standardPackages содержит список стандартных пакетов Go
// Основано на go1.25rc1
var standardPackages = map[string]struct{}{
	"archive/tar":            {},
	"archive/zip":            {},
	"arena":                  {},
	"bufio":                  {},
	"bytes":                  {},
	"cmp":                    {},
	"compress/bzip2":         {},
	"compress/flate":         {},
	"compress/gzip":          {},
	"compress/lzw":           {},
	"compress/zlib":          {},
	"container/heap":         {},
	"container/list":         {},
	"container/ring":         {},
	"context":                {},
	"crypto":                 {},
	"crypto/aes":             {},
	"crypto/boring":          {},
	"crypto/cipher":          {},
	"crypto/des":             {},
	"crypto/dsa":             {},
	"crypto/ecdh":            {},
	"crypto/ecdsa":           {},
	"crypto/ed25519":         {},
	"crypto/elliptic":        {},
	"crypto/fips140":         {},
	"crypto/hkdf":            {},
	"crypto/hmac":            {},
	"crypto/md5":             {},
	"crypto/mlkem":           {},
	"crypto/pbkdf2":          {},
	"crypto/rand":            {},
	"crypto/rc4":             {},
	"crypto/rsa":             {},
	"crypto/sha1":            {},
	"crypto/sha256":          {},
	"crypto/sha3":            {},
	"crypto/sha512":          {},
	"crypto/subtle":          {},
	"crypto/tls":             {},
	"crypto/tls/fipsonly":    {},
	"crypto/x509":            {},
	"crypto/x509/pkix":       {},
	"database/sql":           {},
	"database/sql/driver":    {},
	"debug/buildinfo":        {},
	"debug/dwarf":            {},
	"debug/elf":              {},
	"debug/gosym":            {},
	"debug/macho":            {},
	"debug/pe":               {},
	"debug/plan9obj":         {},
	"embed":                  {},
	"encoding":               {},
	"encoding/ascii85":       {},
	"encoding/asn1":          {},
	"encoding/base32":        {},
	"encoding/base64":        {},
	"encoding/binary":        {},
	"encoding/csv":           {},
	"encoding/gob":           {},
	"encoding/hex":           {},
	"encoding/json":          {},
	"encoding/json/jsontext": {},
	"encoding/json/v2":       {},
	"encoding/pem":           {},
	"encoding/xml":           {},
	"errors":                 {},
	"expvar":                 {},
	"flag":                   {},
	"fmt":                    {},
	"go/ast":                 {},
	"go/build":               {},
	"go/build/constraint":    {},
	"go/constant":            {},
	"go/doc":                 {},
	"go/doc/comment":         {},
	"go/format":              {},
	"go/importer":            {},
	"go/parser":              {},
	"go/printer":             {},
	"go/scanner":             {},
	"go/token":               {},
	"go/types":               {},
	"go/version":             {},
	"hash":                   {},
	"hash/adler32":           {},
	"hash/crc32":             {},
	"hash/crc64":             {},
	"hash/fnv":               {},
	"hash/maphash":           {},
	"html":                   {},
	"html/template":          {},
	"image":                  {},
	"image/color":            {},
	"image/color/palette":    {},
	"image/draw":             {},
	"image/gif":              {},
	"image/jpeg":             {},
	"image/png":              {},
	"index/suffixarray":      {},
	"io":                     {},
	"io/fs":                  {},
	"io/ioutil":              {},
	"iter":                   {},
	"log":                    {},
	"log/slog":               {},
	"log/syslog":             {},
	"maps":                   {},
	"math":                   {},
	"math/big":               {},
	"math/bits":              {},
	"math/cmplx":             {},
	"math/rand":              {},
	"math/rand/v2":           {},
	"mime":                   {},
	"mime/multipart":         {},
	"mime/quotedprintable":   {},
	"net":                    {},
	"net/http":               {},
	"net/http/cgi":           {},
	"net/http/cookiejar":     {},
	"net/http/fcgi":          {},
	"net/http/httptest":      {},
	"net/http/httptrace":     {},
	"net/http/httputil":      {},
	"net/http/pprof":         {},
	"net/mail":               {},
	"net/netip":              {},
	"net/rpc":                {},
	"net/rpc/jsonrpc":        {},
	"net/smtp":               {},
	"net/textproto":          {},
	"net/url":                {},
	"os":                     {},
	"os/exec":                {},
	"os/signal":              {},
	"os/user":                {},
	"path":                   {},
	"path/filepath":          {},
	"plugin":                 {},
	"reflect":                {},
	"regexp":                 {},
	"regexp/syntax":          {},
	"runtime":                {},
	"runtime/cgo":            {},
	"runtime/coverage":       {},
	"runtime/debug":          {},
	"runtime/metrics":        {},
	"runtime/pprof":          {},
	"runtime/race":           {},
	"runtime/trace":          {},
	"slices":                 {},
	"sort":                   {},
	"strconv":                {},
	"strings":                {},
	"structs":                {},
	"sync":                   {},
	"sync/atomic":            {},
	"syscall":                {},
	"syscall/js":             {},
	"testing":                {},
	"testing/fstest":         {},
	"testing/iotest":         {},
	"testing/quick":          {},
	"testing/slogtest":       {},
	"testing/synctest":       {},
	"text/scanner":           {},
	"text/tabwriter":         {},
	"text/template":          {},
	"text/template/parse":    {},
	"time":                   {},
	"time/tzdata":            {},
	"unicode":                {},
	"unicode/utf16":          {},
	"unicode/utf8":           {},
	"unique":                 {},
	"unsafe":                 {},
	"weak":                   {},
}

type importSpec struct {
	start, end int
	name, path string
	original   []byte
}

// FormatImports форматирует импорты в файле согласно эталону:
// 1. Стандартная библиотека (отсортирована)
// 2. Пустая строка
// 3. Локальные пакеты (с префиксом modulePath, отсортированы)
// 4. Пустая строка
// 5. Внешние пакеты (отсортированы)
func FormatImports(src []byte, filename string, modulePath string) ([]byte, error) {

	return formatImports(src, filename, modulePath)
}

// formatImports форматирует импорты в файле согласно эталону:
// 1. Стандартная библиотека (отсортирована)
// 2. Пустая строка
// 3. Локальные пакеты (с префиксом modulePath, отсортированы)
// 4. Пустая строка
// 5. Внешние пакеты (отсортированы)
func formatImports(src []byte, filename string, modulePath string) ([]byte, error) {

	fileSet := token.NewFileSet()
	f, err := parser.ParseFile(fileSet, filename, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	if len(f.Imports) == 0 {
		return src, nil
	}

	// Находим границы блока импортов
	var headEnd, tailStart int
	var hasImports bool

	for _, decl := range f.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			if !hasImports {
				headEnd = int(decl.Pos()) - 1
				hasImports = true
			}
			tailStart = int(decl.End())
		}
	}

	if !hasImports {
		return src, nil
	}

	// Парсим импорты
	var imports []importSpec
	for _, decl := range f.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			for _, spec := range genDecl.Specs {
				imp := spec.(*ast.ImportSpec)
				// Пропускаем import "C"
				if imp.Path.Value == `"C"` {
					continue
				}

				start, end := getImportBounds(imp)
				name := ""
				if imp.Name != nil {
					name = imp.Name.Name
				}
				path := strings.Trim(imp.Path.Value, `"`)

				imports = append(imports, importSpec{
					start:    start,
					end:      end,
					name:     name,
					path:     path,
					original: src[start:end],
				})
			}
		}
	}

	if len(imports) <= 1 {
		return src, nil
	}

	// Определяем модуль из go.mod, если не передан
	localModulePath := modulePath
	if localModulePath == "" {
		localModulePath = findLocalModule(filename)
	}

	// Классифицируем импорты
	var standard, local, external []importSpec

	for _, imp := range imports {
		switch {
		case isStandardPackage(imp.path):
			standard = append(standard, imp)
		case localModulePath != "" && (imp.path == localModulePath || strings.HasPrefix(imp.path, localModulePath+"/")):
			local = append(local, imp)
		default:
			external = append(external, imp)
		}
	}

	// Сортируем каждую группу
	sortImports(standard)
	sortImports(local)
	sortImports(external)

	// Форматируем результат
	// Предварительно выделяем память для body (примерная оценка)
	estimatedBodySize := len(imports) * 50 // примерная оценка: 50 байт на импорт
	body := make([]byte, 0, estimatedBodySize)
	first := true

	// Стандартные импорты
	for _, imp := range standard {
		if !first {
			body = append(body, indent)
		}
		first = false
		// Форматируем импорт правильно
		body = append(body, formatImport(imp)...)
		body = append(body, linebreak)
	}

	// Пустая строка перед локальными
	if len(standard) > 0 && len(local) > 0 {
		body = append(body, linebreak)
		first = true
	}

	// Локальные импорты
	for _, imp := range local {
		if !first {
			body = append(body, indent)
		}
		first = false
		// Форматируем импорт правильно, убираем ненужные алиасы
		body = append(body, formatImportWithoutAlias(imp)...)
		body = append(body, linebreak)
	}

	// Пустая строка перед внешними
	if (len(standard) > 0 || len(local) > 0) && len(external) > 0 {
		body = append(body, linebreak)
		first = true
	}

	// Внешние импорты
	for _, imp := range external {
		if !first {
			body = append(body, indent)
		}
		first = false
		// Форматируем импорт правильно, убираем ненужные алиасы
		body = append(body, formatImportWithoutAlias(imp)...)
		body = append(body, linebreak)
	}

	// Собираем результат
	head := make([]byte, 0, headEnd+20) // предварительно выделяем память
	head = append(head, src[:headEnd]...)
	tail := make([]byte, len(src)-tailStart)
	copy(tail, src[tailStart:])

	// Добавляем начало блока импортов
	head = append(head, []byte("import (")...)
	head = append(head, linebreak)
	// Добавляем конец блока импортов
	body = append(body, []byte{')', linebreak}...)

	// Создаем result с нужной емкостью и копируем данные
	result := make([]byte, 0, len(head)+len(body)+len(tail))
	result = append(result, head...)
	result = append(result, body...)
	result = append(result, tail...)

	// Удаляем \r\n (Windows line breaks)
	result = bytes.ReplaceAll(result, []byte{'\r', '\n'}, []byte{'\n'})

	// Форматируем через go/format
	result, err = format.Source(result)
	if err != nil {
		return nil, fmt.Errorf("format.Source: %w", err)
	}

	return result, nil
}

func getImportBounds(imp *ast.ImportSpec) (start, end int) {

	if imp.Doc != nil {
		start = int(imp.Doc.Pos()) - 1
	} else {
		if imp.Name != nil {
			start = int(imp.Name.Pos()) - 1
		} else {
			start = int(imp.Path.Pos()) - 1
		}
	}

	if imp.Comment != nil {
		end = int(imp.Comment.End())
	} else {
		end = int(imp.Path.End())
	}
	return
}

func isStandardPackage(path string) bool {

	_, ok := standardPackages[path]
	return ok
}

func sortImports(imports []importSpec) {

	sort.Slice(imports, func(i, j int) bool {
		if imports[i].path != imports[j].path {
			return imports[i].path < imports[j].path
		}
		return imports[i].name < imports[j].name
	})
}

func formatImport(imp importSpec) []byte {

	if imp.name != "" {
		return []byte(fmt.Sprintf(`%s "%s"`, imp.name, imp.path))
	}
	return []byte(fmt.Sprintf(`"%s"`, imp.path))
}

func formatImportWithoutAlias(imp importSpec) []byte {

	// ВАЖНО: для внешних пакетов всегда убираем псевдоним, если имя пакета установлено явно
	// Имя пакета определяется из самого пакета (из go/types), а не из пути импорта
	// Импорт должен быть без псевдонима: import "github.com/go-jose/go-jose/v4"
	// Имя пакета используется в коде через Qual, но не должно быть в импорте

	// ВАЖНО: для внешних пакетов всегда убираем псевдоним
	// Имя пакета определяется из самого пакета (из go/types), а не из пути импорта
	// Импорт должен быть без псевдонима: import "github.com/go-jose/go-jose/v4"
	// Имя пакета используется в коде через Qual, но не должно быть в импорте
	return []byte(fmt.Sprintf(`"%s"`, imp.path))
}

func findLocalModule(filename string) string {

	// Ищем go.mod, начиная с директории файла
	dir := filepath.Dir(filename)
	for {
		if dir == "" || dir == "/" {
			return ""
		}
		goModPath := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(goModPath); err == nil {
			// Простой парсинг module path из go.mod
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module"))
					return strings.Trim(modulePath, `"`)
				}
			}
		}
		// Проверяем, не вышли ли за пределы файловой системы
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			break
		}
		dir = parentDir
	}
	return ""
}
