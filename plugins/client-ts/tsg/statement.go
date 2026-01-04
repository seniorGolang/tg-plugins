// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package tsg

import (
	"fmt"
	"strings"
)

// Statement представляет фрагмент TypeScript кода (аналог jen.Code)
type Statement struct {
	code   strings.Builder
	indent int
	export bool // Флаг, что statement должен быть экспортируемым
}

// NewStatement создаёт новый statement
func NewStatement() *Statement {
	return &Statement{
		indent: 0,
	}
}

// String возвращает строковое представление statement
func (s *Statement) String() string {
	result := s.code.String()
	if s.export && result != "" && !strings.HasPrefix(strings.TrimSpace(result), "export ") {
		// Добавляем export перед первым не-комментарием
		lines := strings.Split(result, "\n")
		if len(lines) > 0 {
			// Ищем первую строку, которая не является комментарием
			exportLineIdx := -1
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
					exportLineIdx = i
					break
				}
			}

			if exportLineIdx >= 0 {
				// Находим отступ строки, перед которой нужно добавить export
				originalLine := lines[exportLineIdx]
				indent := len(originalLine) - len(strings.TrimLeft(originalLine, " \t"))
				indentStr := strings.Repeat("    ", indent/4) + strings.Repeat(" ", indent%4)
				// Добавляем export
				lines[exportLineIdx] = indentStr + "export " + strings.TrimLeft(originalLine, " \t")
				result = strings.Join(lines, "\n")
			}
		}
	}
	return result
}

// Add добавляет другой statement
func (s *Statement) Add(other *Statement) *Statement {
	if other != nil {
		s.code.WriteString(other.String())
	}
	return s
}

// Line добавляет пустую строку
func (s *Statement) Line() *Statement {
	s.code.WriteString("\n")
	return s
}

// Id добавляет идентификатор
func (s *Statement) Id(name string) *Statement {
	s.writeIndent()
	s.code.WriteString(name)
	return s
}

// Lit добавляет литерал
func (s *Statement) Lit(value interface{}) *Statement {
	s.writeIndent()
	var str string
	switch v := value.(type) {
	case string:
		str = fmt.Sprintf(`"%s"`, strings.ReplaceAll(strings.ReplaceAll(v, `"`, `\"`), "\n", "\\n"))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		str = fmt.Sprintf("%d", v)
	case float32, float64:
		str = fmt.Sprintf("%g", v)
	case bool:
		str = fmt.Sprintf("%t", v)
	case nil:
		str = "null"
	default:
		str = fmt.Sprintf("%v", v)
	}
	s.code.WriteString(str)
	return s
}

// Comment добавляет комментарий
func (s *Statement) Comment(text string) *Statement {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		s.writeIndent()
		if line != "" {
			s.code.WriteString("// " + line)
		}
		s.code.WriteString("\n")
	}
	return s
}

// Dot добавляет доступ к свойству
func (s *Statement) Dot(property string) *Statement {
	s.code.WriteString("." + property)
	return s
}

// Op добавляет оператор
func (s *Statement) Op(operator string) *Statement {
	s.code.WriteString(" " + operator + " ")
	return s
}

// Call добавляет вызов функции
func (s *Statement) Call(args ...*Statement) *Statement {
	s.code.WriteString("(")
	for i, arg := range args {
		if i > 0 {
			s.code.WriteString(", ")
		}
		if arg != nil {
			s.code.WriteString(arg.String())
		}
	}
	s.code.WriteString(")")
	return s
}

// CallFunc добавляет вызов функции с колбэком
func (s *Statement) CallFunc(fn func(*Group)) *Statement {
	s.code.WriteString("(")
	if fn != nil {
		g := &Group{statement: s}
		fn(g)
	}
	s.code.WriteString(")")
	return s
}

// Type создаёт объявление типа
func (s *Statement) Type(name string) *Statement {
	s.writeIndent()
	s.code.WriteString("type " + name)
	return s
}

// Interface создаёт интерфейс
func (s *Statement) Interface(name string, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("interface " + name)
	s.BlockFunc(fn)
	return s
}

// Class создаёт класс
func (s *Statement) Namespace(name string, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("namespace " + name)
	s.BlockFunc(fn)
	return s
}

func (s *Statement) Class(name string, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("class " + name)
	s.BlockFunc(fn)
	return s
}

// Func создаёт функцию
func (s *Statement) Func(name string) *Statement {
	s.writeIndent()
	s.code.WriteString("function " + name)
	return s
}

// ArrowFunc создаёт стрелочную функцию
func (s *Statement) ArrowFunc(params ...string) *Statement {
	s.writeIndent()
	paramStr := "()"
	if len(params) > 0 {
		paramStr = "(" + strings.Join(params, ", ") + ")"
	}
	s.code.WriteString(paramStr + " =>")
	return s
}

// Async добавляет async
func (s *Statement) Async() *Statement {
	s.writeIndent()
	s.code.WriteString("async ")
	return s
}

// Await добавляет await
func (s *Statement) Await(expr *Statement) *Statement {
	s.writeIndent()
	s.code.WriteString("await ")
	if expr != nil {
		s.code.WriteString(expr.String())
	}
	return s
}

// Return добавляет return
func (s *Statement) Return(value ...*Statement) *Statement {
	s.writeIndent()
	s.code.WriteString("return")
	if len(value) > 0 {
		s.code.WriteString(" ")
		for i, v := range value {
			if i > 0 {
				s.code.WriteString(", ")
			}
			if v != nil {
				s.code.WriteString(v.String())
			}
		}
	} else {
		// Всегда добавляем пробел после return для цепочки вызовов
		s.code.WriteString(" ")
	}
	return s
}

// Assign добавляет присваивание
func (s *Statement) Assign(left, right *Statement) *Statement {
	s.writeIndent()
	if left != nil {
		s.code.WriteString(left.String())
	}
	s.code.WriteString(" = ")
	if right != nil {
		s.code.WriteString(right.String())
	}
	return s
}

// Block создаёт блок кода
func (s *Statement) Block(fn func(*Group)) *Statement {
	s.code.WriteString(" {")
	s.code.WriteString("\n")
	s.indent++
	if fn != nil {
		g := &Group{statement: s, inObject: false}
		fn(g)
	}
	s.indent--
	s.writeIndent()
	s.code.WriteString("}")
	return s
}

// BlockFunc создаёт блок кода (аналог jen.BlockFunc)
func (s *Statement) BlockFunc(fn func(*Group)) *Statement {
	return s.Block(fn)
}

// Params добавляет параметры функции
func (s *Statement) Params(fn func(*Group)) *Statement {
	s.code.WriteString("(")
	if fn != nil {
		g := &Group{statement: s, inParams: true}
		fn(g)
		// Убираем последнюю запятую и пробел если есть
		str := s.code.String()
		if strings.HasSuffix(str, ", )") {
			s.code.Reset()
			s.code.WriteString(strings.TrimSuffix(str, ", )"))
			s.code.WriteString(")")
		} else if strings.HasSuffix(str, " )") {
			s.code.Reset()
			s.code.WriteString(strings.TrimSuffix(str, " )"))
			s.code.WriteString(")")
		}
	}
	s.code.WriteString(")")
	return s
}

// Generic добавляет generic параметры
func (s *Statement) Generic(params ...string) *Statement {
	s.code.WriteString("<" + strings.Join(params, ", ") + ">")
	return s
}

// Extends добавляет extends
func (s *Statement) Extends(base string) *Statement {
	s.code.WriteString(" extends " + base)
	return s
}

// Implements добавляет implements
func (s *Statement) Implements(interfaces ...string) *Statement {
	s.code.WriteString(" implements " + strings.Join(interfaces, ", "))
	return s
}

// Colon добавляет :
func (s *Statement) Colon() *Statement {
	s.code.WriteString(":")
	return s
}

// Semicolon добавляет ;
func (s *Statement) Semicolon() *Statement {
	s.code.WriteString(";")
	return s
}

// Optional добавляет ?
func (s *Statement) Optional() *Statement {
	s.code.WriteString("?")
	return s
}

// Readonly добавляет readonly
func (s *Statement) Readonly() *Statement {
	s.writeIndent()
	s.code.WriteString("readonly ")
	return s
}

// Promise создаёт Promise тип
func (s *Statement) Promise(typeParam *Statement) *Statement {
	s.writeIndent()
	s.code.WriteString("Promise<")
	if typeParam != nil {
		s.code.WriteString(typeParam.String())
	}
	s.code.WriteString(">")
	return s
}

// If создаёт условие
func (s *Statement) If(condition *Statement, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("if (")
	if condition != nil {
		s.code.WriteString(condition.String())
	}
	s.code.WriteString(")")
	s.BlockFunc(fn)
	return s
}

// For создаёт цикл
func (s *Statement) For(init, condition, post *Statement, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("for (")
	parts := []string{}
	if init != nil {
		parts = append(parts, init.String())
	}
	if condition != nil {
		parts = append(parts, condition.String())
	}
	if post != nil {
		parts = append(parts, post.String())
	}
	s.code.WriteString(strings.Join(parts, "; "))
	s.code.WriteString(")")
	s.BlockFunc(fn)
	return s
}

// ForOf создаёт цикл for...of
func (s *Statement) ForOf(variable, iterable string, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("for (const " + variable + " of " + iterable + ")")
	s.BlockFunc(fn)
	return s
}

// Export помечает statement как экспортируемый
func (s *Statement) Export() *Statement {
	s.export = true
	return s
}

// Const создаёт константу
func (s *Statement) Const(name string) *Statement {
	s.writeIndent()
	s.code.WriteString("const " + name)
	return s
}

// Var создаёт переменную
func (s *Statement) Var(name string) *Statement {
	s.writeIndent()
	s.code.WriteString("let " + name)
	return s
}

// writeIndent записывает отступ
func (s *Statement) writeIndent() {
	if s.code.Len() > 0 {
		str := s.code.String()
		if len(str) > 0 {
			lastChar := str[len(str)-1:]
			if lastChar != "\n" && lastChar != " " && lastChar != "{" && lastChar != "(" {
				return
			}
		}
	}
	s.code.WriteString(strings.Repeat("    ", s.indent))
}

// Values создаёт объект/литерал (аналог jen.Values)
func (s *Statement) Values(fn func(*Group)) *Statement {
	s.code.WriteString(" {")
	if fn != nil {
		g := &Group{statement: s, inObject: true}
		fn(g)
		// Убираем последнюю запятую если есть
		str := s.code.String()
		if strings.HasSuffix(str, ",\n }") {
			s.code.Reset()
			s.code.WriteString(strings.TrimSuffix(str, ",\n }"))
			s.code.WriteString("\n }")
		}
	}
	s.code.WriteString(" }")
	return s
}

// Throw создаёт throw
func (s *Statement) Throw(expr *Statement) *Statement {
	s.writeIndent()
	s.code.WriteString("throw ")
	if expr != nil {
		s.code.WriteString(expr.String())
	}
	s.code.WriteString(";")
	return s
}

// New создаёт new
func (s *Statement) New(typeName string) *Statement {
	s.code.WriteString("new " + typeName)
	return s
}

// This добавляет this
func (s *Statement) This() *Statement {
	s.code.WriteString("this")
	return s
}

// Private добавляет private
func (s *Statement) Private() *Statement {
	s.writeIndent()
	s.code.WriteString("private ")
	return s
}

// Public добавляет public (явно, хотя по умолчанию в TS всё public)
func (s *Statement) Public() *Statement {
	s.writeIndent()
	s.code.WriteString("public ")
	return s
}

// Method создаёт метод класса
func (s *Statement) Method(name string, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString(name)
	s.code.WriteString("(")
	s.code.WriteString(")")
	s.code.WriteString(": ")
	s.BlockFunc(fn)
	return s
}

// AsyncMethod создаёт async метод класса
func (s *Statement) AsyncMethod(name string, returnType *Statement, fn func(*Group)) *Statement {
	return s.AsyncMethodWithParams(name, nil, returnType, fn)
}

// AsyncMethodWithParams создаёт async метод класса с параметрами
func (s *Statement) AsyncMethodWithParams(name string, params *Statement, returnType *Statement, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("async " + name)
	if params != nil {
		s.code.WriteString(params.String())
	} else {
		s.code.WriteString("()")
	}
	if returnType != nil {
		s.code.WriteString(": Promise<")
		s.code.WriteString(returnType.String())
		s.code.WriteString(">")
	}
	s.code.WriteString(" {")
	s.code.WriteString("\n")
	s.indent++
	if fn != nil {
		g := &Group{statement: s, inObject: false}
		fn(g)
	}
	s.indent--
	s.writeIndent()
	s.code.WriteString("}")
	return s
}

// AsyncMethodWithGeneric создаёт async метод класса с generic параметрами
func (s *Statement) AsyncMethodWithGeneric(name string, genericParams *Statement, params *Statement, returnType *Statement, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("async " + name)
	if genericParams != nil {
		s.code.WriteString(genericParams.String())
	}
	if params != nil {
		s.code.WriteString(params.String())
	} else {
		s.code.WriteString("()")
	}
	if returnType != nil {
		s.code.WriteString(": Promise<")
		s.code.WriteString(returnType.String())
		s.code.WriteString(">")
	}
	s.code.WriteString(" {")
	s.code.WriteString("\n")
	s.indent++
	if fn != nil {
		g := &Group{statement: s, inObject: false}
		fn(g)
	}
	s.indent--
	s.writeIndent()
	s.code.WriteString("}")
	return s
}

// Constructor создаёт конструктор класса
func (s *Statement) Constructor(fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("constructor(")
	s.code.WriteString(")")
	s.code.WriteString(" {")
	s.code.WriteString("\n")
	s.indent++
	if fn != nil {
		g := &Group{statement: s, inObject: false}
		fn(g)
	}
	s.indent--
	s.writeIndent()
	s.code.WriteString("}")
	return s
}

// TypeAlias создаёт type alias (type X = ...)
func (s *Statement) TypeAlias(name string) *Statement {
	s.writeIndent()
	s.code.WriteString("type " + name + " =")
	return s
}

// Record создаёт Record<K, V> тип
func (s *Statement) Record(key, value *Statement) *Statement {
	s.code.WriteString("Record<")
	if key != nil {
		s.code.WriteString(key.String())
	}
	s.code.WriteString(", ")
	if value != nil {
		s.code.WriteString(value.String())
	}
	s.code.WriteString(">")
	return s
}

// Array создаёт массив тип (T[])
func (s *Statement) Array(elem *Statement) *Statement {
	if elem != nil {
		s.code.WriteString(elem.String())
	}
	s.code.WriteString("[]")
	return s
}

// Index создаёт литерал массива []
func (s *Statement) Index(elem *Statement) *Statement {
	s.code.WriteString("[")
	if elem != nil {
		s.code.WriteString(elem.String())
	}
	s.code.WriteString("]")
	return s
}

// Arrow создаёт стрелочную функцию тип ((params) => returnType)
func (s *Statement) Arrow(params, returnType *Statement) *Statement {
	s.code.WriteString("(")
	if params != nil {
		s.code.WriteString(params.String())
	}
	s.code.WriteString(") => ")
	if returnType != nil {
		s.code.WriteString(returnType.String())
	}
	return s
}

// Union создаёт union тип (Type1 | Type2 | ...)
func (s *Statement) Union(types ...*Statement) *Statement {
	for i, t := range types {
		if i > 0 {
			s.code.WriteString(" | ")
		}
		if t != nil {
			s.code.WriteString(t.String())
		}
	}
	return s
}

// Nullable создаёт nullable тип (Type | null)
func (s *Statement) Nullable(baseType *Statement) *Statement {
	if baseType != nil {
		s.code.WriteString(baseType.String())
	}
	s.code.WriteString(" | null")
	return s
}

// ReadonlyArray создаёт readonly массив (readonly Type[])
func (s *Statement) ReadonlyArray(elemType *Statement) *Statement {
	s.code.WriteString("readonly ")
	if elemType != nil {
		s.code.WriteString(elemType.String())
	}
	s.code.WriteString("[]")
	return s
}

// ObjectLiteral создаёт объектный литерал { field1: value1, field2: value2, ... }
func (s *Statement) ObjectLiteral(fn func(*Group)) *Statement {
	s.code.WriteString("{")
	if fn != nil {
		s.code.WriteString("\n")
		s.indent++
		g := &Group{statement: s, inObject: true}
		fn(g)
		s.indent--
		s.writeIndent()
	}
	s.code.WriteString("}")
	return s
}

// ObjectField добавляет поле в объектный литерал (name: value)
func (s *Statement) ObjectField(name string, value *Statement) *Statement {
	s.writeIndent()
	// Если имя содержит дефис или другие специальные символы, заключаем в кавычки
	if strings.Contains(name, "-") || strings.Contains(name, " ") || strings.ContainsAny(name, "!@#$%^&*()+={}[]|\\:;\"'<>?,./") {
		s.code.WriteString(`"` + name + `"`)
	} else {
		s.code.WriteString(name)
	}
	s.code.WriteString(": ")
	if value != nil {
		s.code.WriteString(value.String())
	}
	return s
}

// OptionalField добавляет опциональное поле в объектный литерал (name?: value)
func (s *Statement) OptionalField(name string, value *Statement) *Statement {
	s.writeIndent()
	s.code.WriteString(name)
	s.code.WriteString("?: ")
	if value != nil {
		s.code.WriteString(value.String())
	}
	return s
}

// Spread создаёт spread оператор (...obj)
func (s *Statement) Spread(expr *Statement) *Statement {
	s.code.WriteString("...")
	if expr != nil {
		s.code.WriteString(expr.String())
	}
	return s
}

// Try создаёт try-catch блок
func (s *Statement) Try(tryFn func(*Group), catchFn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("try {")
	s.code.WriteString("\n")
	s.indent++
	if tryFn != nil {
		g := &Group{statement: s, inObject: false}
		tryFn(g)
	}
	s.indent--
	s.writeIndent()
	s.code.WriteString("} catch (e) {")
	s.code.WriteString("\n")
	s.indent++
	if catchFn != nil {
		g := &Group{statement: s, inObject: false}
		catchFn(g)
	}
	s.indent--
	s.writeIndent()
	s.code.WriteString("}")
	return s
}

// Typeof создаёт typeof проверку (typeof expr === "type")
func (s *Statement) Typeof(expr *Statement, typeStr string) *Statement {
	s.code.WriteString("typeof ")
	if expr != nil {
		s.code.WriteString(expr.String())
	}
	s.code.WriteString(" === ")
	s.code.WriteString(`"` + typeStr + `"`)
	return s
}

// In создаёт проверку in (prop in obj)
func (s *Statement) In(prop string, obj *Statement) *Statement {
	s.code.WriteString(`"` + prop + `"`)
	s.code.WriteString(" in ")
	if obj != nil {
		s.code.WriteString(obj.String())
	}
	return s
}

// TemplateString создаёт template string (`text ${expr} text`)
func (s *Statement) TemplateString(parts []string, exprs []*Statement) *Statement {
	s.code.WriteString("`")
	for i, part := range parts {
		s.code.WriteString(part)
		if i < len(exprs) && exprs[i] != nil {
			s.code.WriteString("${")
			s.code.WriteString(exprs[i].String())
			s.code.WriteString("}")
		}
	}
	s.code.WriteString("`")
	return s
}

// GenericWithDefault создаёт generic параметр с default значением (<T = Default>)
func (s *Statement) GenericWithDefault(params map[string]string) *Statement {
	s.code.WriteString("<")
	first := true
	for name, defaultValue := range params {
		if !first {
			s.code.WriteString(", ")
		}
		s.code.WriteString(name)
		if defaultValue != "" {
			s.code.WriteString(" = ")
			s.code.WriteString(defaultValue)
		}
		first = false
	}
	s.code.WriteString(">")
	return s
}

// ExportClass создаёт экспортируемый класс
func (s *Statement) ExportClass(name string, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("export class " + name)
	s.BlockFunc(fn)
	return s
}

// ExportClassWithGeneric создаёт экспортируемый класс с generic параметрами
func (s *Statement) ExportClassWithGeneric(name string, genericParams *Statement, fn func(*Group)) *Statement {
	s.writeIndent()
	s.code.WriteString("export class " + name)
	if genericParams != nil {
		s.code.WriteString(genericParams.String())
	}
	s.BlockFunc(fn)
	return s
}

// Void создаёт void тип
func (s *Statement) Void() *Statement {
	s.code.WriteString("void")
	return s
}

// Any создаёт any тип
func (s *Statement) Any() *Statement {
	s.code.WriteString("any")
	return s
}

// Never создаёт never тип
func (s *Statement) Never() *Statement {
	s.code.WriteString("never")
	return s
}

// Import создаёт import statement (import ... from 'path')
func (s *Statement) Import(imports string, path string) *Statement {
	s.writeIndent()
	s.code.WriteString("import ")
	s.code.WriteString(imports)
	s.code.WriteString(" from ")
	s.code.WriteString("'" + path + "'")
	s.code.WriteString(";")
	return s
}

// ImportAll создаёт import * as statement (import * as alias from 'path')
func (s *Statement) ImportAll(alias string, path string) *Statement {
	s.writeIndent()
	s.code.WriteString("import * as ")
	s.code.WriteString(alias)
	s.code.WriteString(" from ")
	s.code.WriteString("'" + path + "'")
	s.code.WriteString(";")
	return s
}

// TypeFromString создаёт statement из строки типа TypeScript
// Используется для вставки типов, полученных из typeLink()
// ВНИМАНИЕ: Используйте только для типов, полученных из typeLink(), не для генерации структуры кода!
func TypeFromString(typeStr string) *Statement {
	stmt := NewStatement()
	stmt.code.WriteString(typeStr)
	return stmt
}

// Group представляет группу statement'ов (аналог jen.Group)
type Group struct {
	statement *Statement
	inObject  bool // Флаг, что мы внутри объекта (для правильной расстановки запятых)
	inParams  bool // Флаг, что мы внутри параметров функции (для правильной расстановки запятых)
}

// Add добавляет statement в группу
func (g *Group) Add(stmt *Statement) {
	if stmt != nil {
		stmtStr := stmt.String()
		stmtTrimmed := strings.TrimSpace(stmtStr)
		currentStr := g.statement.code.String()
		
		// Для spread оператора в объектах не добавляем отступ и обрабатываем запятую отдельно
		isSpreadInObject := g.inObject && strings.HasPrefix(stmtTrimmed, "...")
		
		if !g.inParams && !isSpreadInObject {
			g.statement.writeIndent()
		}

		// Для параметров функции добавляем запятую перед каждым параметром кроме первого
		if g.inParams {
			if currentStr != "" && !strings.HasSuffix(currentStr, "(") && !strings.HasSuffix(currentStr, ", ") && !strings.HasSuffix(currentStr, " ") {
				g.statement.code.WriteString(", ")
			}
		} else if g.inObject {
			// В объектах TypeScript поля разделяются запятыми
			// Проверяем, не первое ли это поле в объекте
			// Ищем последнюю открывающую скобку и проверяем, что после неё ничего нет
			lastBraceIdx := strings.LastIndex(currentStr, "{")
			if lastBraceIdx >= 0 {
				afterBrace := currentStr[lastBraceIdx+1:]
				afterBrace = strings.TrimSpace(afterBrace)
				// Если после { есть что-то кроме пробелов и переносов строк, значит это не первое поле
				if afterBrace != "" && afterBrace != "\n" {
					// Для spread оператора добавляем запятую в конец предыдущей строки
					if isSpreadInObject {
						// Получаем актуальное состояние кода (после writeIndent, если он был вызван)
						currentStr = g.statement.code.String()
						// Находим последнюю строку и добавляем запятую в её конец
						lastNewlineIdx := strings.LastIndex(currentStr, "\n")
						if lastNewlineIdx >= 0 {
							lastLine := currentStr[lastNewlineIdx+1:]
							lastLineTrimmed := strings.TrimSpace(lastLine)
							// Всегда добавляем запятую, если последняя строка не пустая и не заканчивается запятой
							if lastLineTrimmed != "" && !strings.HasSuffix(lastLineTrimmed, ",") {
								// Перезаписываем код: всё до последней строки + последняя строка + запятая + пробел
								codeBeforeLastLine := currentStr[:lastNewlineIdx+1]
								g.statement.code.Reset()
								g.statement.code.WriteString(codeBeforeLastLine)
								g.statement.code.WriteString(lastLine)
								g.statement.code.WriteString(",")
								g.statement.code.WriteString(" ")
							} else if lastLineTrimmed == "" {
								// Если последняя строка пустая (только перенос), ищем предпоследнюю
								if lastNewlineIdx > 0 {
									prevNewlineIdx := strings.LastIndex(currentStr[:lastNewlineIdx], "\n")
									if prevNewlineIdx >= 0 {
										prevLine := currentStr[prevNewlineIdx+1:lastNewlineIdx]
										prevLineTrimmed := strings.TrimSpace(prevLine)
										if prevLineTrimmed != "" && !strings.HasSuffix(prevLineTrimmed, ",") {
											codeBeforePrevLine := currentStr[:prevNewlineIdx+1]
											g.statement.code.Reset()
											g.statement.code.WriteString(codeBeforePrevLine)
											g.statement.code.WriteString(prevLine)
											g.statement.code.WriteString(",")
											g.statement.code.WriteString("\n")
											g.statement.code.WriteString(strings.Repeat("    ", g.statement.indent))
										}
									}
								}
							}
						} else {
							// Если нет переноса строки, просто добавляем запятую и пробел
							trimmed := strings.TrimSpace(currentStr)
							if trimmed != "" && !strings.HasSuffix(trimmed, ",") {
								g.statement.code.WriteString(",")
								g.statement.code.WriteString(" ")
							}
						}
					} else if !strings.HasSuffix(stmtStr, ";") && !strings.HasSuffix(stmtStr, ",") {
						// Для обычных полей добавляем запятую в начале новой строки
						g.statement.code.WriteString(",")
					}
				}
			}
		}

		g.statement.code.WriteString(stmtStr)
		if !g.inParams {
			// Не добавляем перенос строки перед spread оператором в объектах
			stmtTrimmed := strings.TrimSpace(stmtStr)
			if !g.inObject || !strings.HasPrefix(stmtTrimmed, "...") {
				g.statement.code.WriteString("\n")
			}
		}
	}
}

// Id создаёт идентификатор и добавляет в группу
func (g *Group) Id(name string) *Statement {
	stmt := NewStatement()
	stmt.indent = g.statement.indent
	stmt.writeIndent()
	stmt.code.WriteString(name)
	return stmt
}

// Lit создаёт литерал и добавляет в группу
func (g *Group) Lit(value interface{}) *Statement {
	stmt := NewStatement()
	stmt.indent = g.statement.indent
	stmt.writeIndent()
	var str string
	switch v := value.(type) {
	case string:
		str = fmt.Sprintf(`"%s"`, strings.ReplaceAll(strings.ReplaceAll(v, `"`, `\"`), "\n", "\\n"))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		str = fmt.Sprintf("%d", v)
	case float32, float64:
		str = fmt.Sprintf("%g", v)
	case bool:
		str = fmt.Sprintf("%t", v)
	case nil:
		str = "null"
	default:
		str = fmt.Sprintf("%v", v)
	}
	stmt.code.WriteString(str)
	return stmt
}

// Comment добавляет комментарий
func (g *Group) Comment(text string) {
	g.statement.Comment(text)
}

// Line добавляет пустую строку
func (g *Group) Line() {
	g.statement.Line()
}

// Return добавляет return
func (g *Group) Return(value ...*Statement) {
	g.statement.writeIndent()
	g.statement.code.WriteString("return")
	if len(value) > 0 {
		g.statement.code.WriteString(" ")
		for i, v := range value {
			if i > 0 {
				g.statement.code.WriteString(", ")
			}
			if v != nil {
				g.statement.code.WriteString(v.String())
			}
		}
	}
	g.statement.code.WriteString(";\n")
}

// Assign добавляет присваивание
func (g *Group) Assign(left, right *Statement) {
	g.statement.writeIndent()
	if left != nil {
		g.statement.code.WriteString(left.String())
	}
	g.statement.code.WriteString(" = ")
	if right != nil {
		g.statement.code.WriteString(right.String())
	}
	g.statement.code.WriteString(";\n")
}

// If добавляет условие
func (g *Group) If(condition *Statement, fn func(*Group)) {
	g.statement.writeIndent()
	g.statement.code.WriteString("if (")
	if condition != nil {
		g.statement.code.WriteString(condition.String())
	}
	g.statement.code.WriteString(") {")
	g.statement.code.WriteString("\n")
	g.statement.indent++
	if fn != nil {
		fn(g)
	}
	g.statement.indent--
	g.statement.writeIndent()
	g.statement.code.WriteString("}\n")
}

// Throw добавляет throw
func (g *Group) Throw(expr *Statement) {
	g.statement.writeIndent()
	g.statement.code.WriteString("throw ")
	if expr != nil {
		g.statement.code.WriteString(expr.String())
	}
	g.statement.code.WriteString(";\n")
}

// Try добавляет try-catch блок
func (g *Group) Try(tryFn func(*Group), catchFn func(*Group)) {
	g.statement.writeIndent()
	g.statement.code.WriteString("try {")
	g.statement.code.WriteString("\n")
	g.statement.indent++
	if tryFn != nil {
		tryFn(g)
	}
	g.statement.indent--
	g.statement.writeIndent()
	g.statement.code.WriteString("} catch (e) {")
	g.statement.code.WriteString("\n")
	g.statement.indent++
	if catchFn != nil {
		catchFn(g)
	}
	g.statement.indent--
	g.statement.writeIndent()
	g.statement.code.WriteString("}\n")
}
