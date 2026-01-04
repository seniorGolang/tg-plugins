package goimports

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	Name string
	In   io.Reader
	Out  io.Writer
}

type Runner struct {
	files []File
}

func New(path ...string) (runner Runner, err error) {

	runner.files, err = buildFiles(path...)
	return
}

func NewFromFile(path string) (runner Runner, err error) {

	runner.files, err = buildFile(path)
	return
}

func NewFromFiles(files ...File) Runner {

	return Runner{
		files: files,
	}
}

func (r Runner) Run(modulePath string) (err error) {

	for _, file := range r.files {
		if err = r.processFile(file, modulePath); err != nil {
			return
		}
	}
	return
}

func (r Runner) processFile(file File, modulePath string) (err error) {

	var src []byte
	if file.In == nil {
		if src, err = os.ReadFile(file.Name); err != nil {
			return
		}
	} else {
		if src, err = io.ReadAll(file.In); err != nil {
			return
		}
	}

	// Используем собственную реализацию форматирования импортов
	// Порядок: стандартная библиотека -> локальные пакеты -> внешние пакеты
	res, err := formatImports(src, file.Name, modulePath)
	if err != nil {
		// Если форматирование не удалось, возвращаем исходный код без изменений
		err = nil
		return
	}

	if len(res) == 0 {
		// Если результат пустой, возвращаем исходный код
		return
	}

	if bytes.Equal(src, res) {
		if s, ok := file.In.(io.Seeker); ok {
			_, err = s.Seek(0, 0)
		}
		return
	}

	if file.Out == nil {
		return os.WriteFile(file.Name, res, 0)
	}

	_, err = file.Out.Write(res)
	if c, ok := file.Out.(io.Closer); ok {
		_ = c.Close()
	}
	return
}

func isGoFile(f os.FileInfo) bool {

	// ignore non-Go files
	name := f.Name()
	return !f.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
}

func buildFiles(paths ...string) (files []File, err error) {

	for _, root := range paths {
		err = filepath.Walk(root, func(path string, info os.FileInfo, _ error) (err error) {
			if info == nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if !isGoFile(info) {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files = append(files, File{
				Name: path,
				In:   bytes.NewReader(b),
			})
			return
		})
		if err != nil {
			return
		}
	}
	return
}

func buildFile(path string) (files []File, err error) {

	info, _ := os.Stat(path)
	if info == nil {
		return files, nil
	}
	if info.IsDir() {
		return files, nil
	}
	if !isGoFile(info) {
		return files, nil
	}
	var b []byte
	if b, err = os.ReadFile(path); err != nil {
		return
	}
	files = append(files, File{
		Name: path,
		In:   bytes.NewReader(b),
	})
	return
}

// GetModulePath возвращает путь модуля из go.mod файла, начиная с указанного пути.
func GetModulePath(filePath string) string {

	// Ищем go.mod, начиная с директории файла
	dir := filepath.Dir(filePath)
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
