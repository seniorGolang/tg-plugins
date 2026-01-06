// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package parser

import (
	"fmt"
	"go/types"
	"log/slog"
)

// detectInterfaces определяет все интерфейсы, которые реализует тип.
// Проверяет как сам тип, так и указатель на тип, так как некоторые интерфейсы
// могут быть реализованы только через указатель (например, TextUnmarshaler).
func detectInterfaces(log *slog.Logger, typ types.Type, coreType *Type, project *Project) {
	if coreType.ImportPkgPath == "" || coreType.TypeName == "" {
		return
	}

	allInterfaces := getAllInterfaces(log, project)
	if len(allInterfaces) == 0 {
		return
	}

	implements := make([]string, 0)
	seenIDs := make(map[string]bool)

	// Проверяем сам тип
	for ifaceID, iface := range allInterfaces {
		if types.Implements(typ, iface) {
			if !seenIDs[ifaceID] {
				implements = append(implements, ifaceID)
				seenIDs[ifaceID] = true
			}
		}
	}

	// Проверяем указатель на тип (для интерфейсов, которые требуют pointer receiver)
	// Это нужно для типов, которые реализуют интерфейсы только через указатель
	pointerType := types.NewPointer(typ)
	for ifaceID, iface := range allInterfaces {
		if types.Implements(pointerType, iface) {
			if !seenIDs[ifaceID] {
				implements = append(implements, ifaceID)
				seenIDs[ifaceID] = true
			}
		}
	}

	coreType.ImplementsInterfaces = implements
}

// getAllInterfaces собирает все уникальные интерфейсы из всех загруженных пакетов.
// Возвращает map с ключом в формате "pkgPath:interfaceName" и значением *types.Interface.
func getAllInterfaces(log *slog.Logger, project *Project) map[string]*types.Interface {
	globalPackageCache.mu.RLock()
	defer globalPackageCache.mu.RUnlock()

	interfaces := make(map[string]*types.Interface)
	seenInterfaces := make(map[string]bool)

	for _, pkgInfo := range globalPackageCache.cache {
		if pkgInfo.Types == nil {
			continue
		}

		scope := pkgInfo.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}

			typeName, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}

			typ := typeName.Type()
			iface, ok := typ.Underlying().(*types.Interface)
			if !ok {
				continue
			}

			// Формируем уникальный ID для интерфейса
			pkgPath := typeName.Pkg().Path()
			interfaceID := fmt.Sprintf("%s:%s", pkgPath, typeName.Name())

			// Проверяем, не добавляли ли мы уже этот интерфейс по ID
			// Если ID уже есть, это дубликат - пропускаем
			if seenInterfaces[interfaceID] {
				continue
			}

			// Добавляем интерфейс
			interfaces[interfaceID] = iface
			seenInterfaces[interfaceID] = true
		}
	}

	return interfaces
}
