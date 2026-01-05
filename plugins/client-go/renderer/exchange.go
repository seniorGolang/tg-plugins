// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/core"
)

// RenderExchange генерирует файл exchange для контракта.
func (r *ClientRenderer) RenderExchange(contract *core.Contract) error {

	outDir := r.outDir
	pkgName := filepath.Base(outDir)
	srcFile := NewSrcFile(pkgName)
	srcFile.PackageComment(DoNotEdit)

	ctx := context.WithValue(context.Background(), keyCode, srcFile) // nolint
	ctx = context.WithValue(ctx, keyPackage, pkgName)                // nolint

	for _, method := range contract.Methods {
		srcFile.Add(r.exchange(ctx, contract, r.requestStructName(contract, method), r.fieldsArgument(method))).Line()
		srcFile.Add(r.exchange(ctx, contract, r.responseStructName(contract, method), r.fieldsResult(method))).Line()
	}
	return srcFile.Save(path.Join(outDir, strings.ToLower(contract.Name)+"-exchange.go"))
}

func (r *ClientRenderer) exchange(ctx context.Context, contract *core.Contract, name string, fields []exchangeField) Code {

	if len(fields) == 0 {
		return Comment("Formal exchange type, please do not delete.").Line().Type().Id(name).Struct()
	}
	template := "%s,omitempty"
	if r.contains(contract.Annotations, "tagNoOmitempty") {
		template = "%s"
	}
	return Type().Id(name).StructFunc(func(gr *Group) {
		for _, field := range fields {
			fieldCode := r.structField(ctx, field, template)
			gr.Add(fieldCode)
		}
	})
}

func (r *ClientRenderer) structField(ctx context.Context, field exchangeField, template string) *Statement {

	var isInlined bool
	tags := map[string]string{"json": fmt.Sprintf(template, field.name)}
	for tag, value := range field.tags {
		if tag == "json" {
			if strings.Contains(value, "inline") {
				isInlined = true
			}
		}
		tags[tag] = value
	}
	var s *Statement
	if isInlined {
		// Для inline используем версию fieldType, которая использует локальные типы
		s = r.fieldType(ctx, field.typeID, field.numberOfPointers, false)
		s.Tag(map[string]string{"json": ",inline"})
	} else {
		s = Id(ToCamel(field.name))
		// Проверяем, есть ли информация о массивах/map
		if field.isSlice || field.arrayLen > 0 || field.mapKeyID != "" {
			// Создаем временный Variable для передачи в fieldTypeFromVariable
			v := &core.Variable{
				TypeID:           field.typeID,
				NumberOfPointers: field.numberOfPointers,
				IsSlice:          field.isSlice,
				ArrayLen:         field.arrayLen,
				IsEllipsis:       field.isEllipsis,
				ElementPointers:  field.elementPointers,
				MapKeyID:         field.mapKeyID,
				MapValueID:       field.mapValueID,
			}
			s.Add(r.fieldTypeFromVariable(ctx, v, false))
		} else {
			// ВАЖНО: используем fieldType, чтобы использовать локальные типы из dto пакета клиента
			s.Add(r.fieldType(ctx, field.typeID, field.numberOfPointers, false))
		}
		s.Tag(tags)
	}
	if field.isEllipsis {
		s.Comment("This field was defined with ellipsis (...).")
	}
	return s
}
