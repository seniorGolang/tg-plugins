## Что такое плагин?

Плагин — это WASM (WebAssembly) модуль, который расширяет функциональность приложения `tg`.

Плагины позволяют:

- Добавлять новые команды и возможности
- Автоматизировать повторяющиеся задачи
- Интегрироваться с внешними сервисами
- Кастомизировать поведение приложения под конкретные нужды
- Генерировать код
- Генерировать документацию
- Делать любые манипуляции на основе описания проекта и его контрактов

Плагины компилируются в `.tgp` файлы и выполняются в изолированной WASM среде для безопасности.

## Манифест плагина

Манифест — это файл `plugin.json` в директории плагина (`plugins/{plugin-name}/plugin.json`), который содержит метаданные о плагине.

### Формат манифеста

```json
{
  "name": "server",
  "version": "1.0.0",
  "description": "Плагин server",
  "author": "Rick Sanchez <rick.sanchez@example.com>",
  "license": "MIT"
}
```

### Обязательные поля

- **`name`** — должно совпадать с именем директории в `plugins/{plugin-name}/`
- **`version`** — должна совпадать с версией в теге при публикации
- **`description`** — краткое описание назначения плагина
- **`author`** — имя автора или организации
- **`license`** — лицензия плагина

## Реализация логики плагина

Основная логика плагина реализуется в файле `plugins/server/plugin.go`.

### Структура плагина

Плагин должен реализовать интерфейс `Plugin`:

```go
type Plugin interface {
    Info() PluginInfo
    Execute(project Project, rootDir string, options map[string]any) (err error)
}
```

### Метод `Info()`

Возвращает метаданные плагина:

```go
func (p *ServerPlugin) Info() shared.PluginInfo {
    return shared.PluginInfo{
        Name:           "server",
        Version:        "1.0.0",
        Description:    "Плагин server",
        Author:         "Rick Sanchez <rick.sanchez@example.com>",
        License:        "MIT",
        Category:       "utility",
        Commands: []shared.Command{
            {
                Path:        []string{"server"},
                Description: "Плагин server",
                Options:     []shared.Option{},
            },
        },
        AllowedHTTP:     []string{}, // Белый список адресов для HTTP запросов (включая, regex шаблоны)
        AllowedShellCMDs: []string{}, // Белый список команд для выполнения через хост (например: ["go", "git"])
    }
}
```

### Метод `Execute()`

Содержит основную логику плагина:

```go
func (p *ServerPlugin) Execute(project shared.Project, rootDir string, options map[string]any, path ...string) (err error) {
    logger := shared.GetLogger()
    
    // Ваша логика здесь
    // - Работа с файлами через os.ReadFile(), os.WriteFile()
    // - HTTP запросы через shared.GetHTTPClient()
    // - Логирование через logger.Info(), logger.Error() и т.д.
    
    return nil
}
```

### Доступные функции

- **Логирование**: `shared.GetLogger()` — возвращает логгер для записи сообщений
- **HTTP запросы**: `shared.GetHTTPClient()` — HTTP клиент для запросов на разрешённые домены
- **Работа с файлами**: стандартные Go функции `os.ReadFile()`, `os.WriteFile()`, `os.Open()` и т.д. через WASI

### Пример реализации

```go
func (p *ServerPlugin) Execute(project shared.Project, rootDir string, options map[string]any, path ...string) (err error) {
    logger := shared.GetLogger()
    logger.Info("Начинаю выполнение плагина")
    
    // Читаем файл
    data, err := os.ReadFile(filepath.Join(rootDir, "config.json"))
    if err != nil {
        return fmt.Errorf("не удалось прочитать файл: %w", err)
    }
    
    // Обрабатываем данные
    // ...
    
    // Записываем результат
    if err := os.WriteFile(filepath.Join(rootDir, "output.txt"), result, 0644); err != nil {
        return fmt.Errorf("не удалось записать файл: %w", err)
    }
    
    logger.Info("Плагин успешно выполнен")
    return
}
```

## Добавление нового плагина в репозиторий

Если в репозитории уже есть плагины, для добавления нового используйте команду:

```bash
tg plugin add \
  --name new-plugin \
  --command new-cmd \
  --dir plugins/new-plugin \
  --license MIT
```

Команда создаст:
- `plugins/new-plugin/plugin.go` — основной код плагина
- `plugins/new-plugin/init.go` — инициализация плагина

После создания:
1. Реализуйте логику в `plugin.go`
2. Обновите `plugin.json` в директории плагина (если нужно)
3. Протестируйте плагин: `tg plugin build` из директории плагина

## Ограничения плагина

### Что можно делать

✅ **Можно**:
- Использовать стандартные библиотеки Go (strings, json, encoding и т.д.)
- Работать с файлами через `os.ReadFile()`, `os.WriteFile()`, `os.Open()` и т.д. (через WASI)
- Делать HTTP запросы через `shared.GetHTTPClient()` на адреса из `AllowedHTTP`
- Использовать логирование через `shared.GetLogger()`
- Обрабатывать данные проекта из параметра `Execute()`

### Что нельзя делать

❌ **Нельзя**:
- Использовать `net/http` напрямую — только через `shared.GetHTTPClient()`
- Выполнять системные команды через `exec` — нет доступа к процессам
- Использовать `runtime.Gosched()` или другие примитивы синхронизации — WASM однопоточный
- Использовать горутины для параллелизма — WASM однопоточный, горутины выполняются асинхронно
- Делать HTTP запросы на адреса, не указанные в `AllowedHTTP`
- Получать доступ к файлам вне проекта — файловая система изолирована через WASI

### Особенности работы

1. **Однопоточность**: WASM выполняется в одном потоке, горутины технически можно создавать, но они не дают параллелизма и выполняются асинхронно
2. **Изоляция файловой системы**: Доступ только к файлам проекта через WASI (монтируется через `os.DirFS(rootDir)`)
3. **Безопасность**: Все внешние операции контролируются хостом
4. **Память**: Память управляется автоматически, но избегайте частых вызовов функций хоста в циклах
5. **Доступные системные функции**:
   - **Системное время** — доступ к системному времени (wall clock time)
   - **Монотонное время** — доступ к монотонному времени в наносекундах
   - **Задержки** — возможность использовать функции задержки/сна
   - **Генератор случайных чисел** — доступ к криптографически стойкому генератору случайных чисел
   - **Файловая система** — доступ к файлам проекта через WASI (монтируется через `os.DirFS(rootDir)`)
   - **Инициализация** — автоматическая инициализация при загрузке модуля

## Публикация новой версии


### Публикация в GitHub

#### Подготовка

1. Обновите версию в `plugins/server/plugin.json`:
   ```json
   {
     "version": "1.2.3"
   }
   ```

2. Закоммитьте и запушьте изменения:
   ```bash
   git add plugins/server/plugin.json
   git commit -m "Release v1.2.3"
   git push origin main
   ```

#### Формат тега

GitHub Actions поддерживает формат тега: `v{version}`
   - Пример: `v1.2.3`
   - Публикует **все** плагины в репозитории с одной версией

#### Процесс публикации

```bash
# Создайте и запушьте тег
git tag v1.2.3
git push origin v1.2.3
```

#### Что происходит автоматически

После пуша тега GitHub Actions автоматически:

1. **Собирает все плагины**:
   - Компилирует каждый плагин в WASM модуль (`.tgp`)
   - Генерирует SHA256 checksum для каждого

2. **Создаёт GitHub Release**:
   - Создаёт релиз с тегом `v1.2.3`
   - Создаёт `manifest.json` со списком всех плагинов
   - Прикрепляет файлы: для каждого плагина `{plugin-name}.json`, `{plugin-name}.tgp`, `{plugin-name}.sha256`

#### Доступ к плагину

После публикации плагины доступны в разделе Releases:
```
https://github.com/{owner}/{repo}/releases/tag/v{version}
```

Файлы можно скачать напрямую из релиза.

