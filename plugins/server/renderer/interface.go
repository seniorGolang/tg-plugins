// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

// Renderer определяет интерфейс для рендеринга различных компонентов сервера.
type Renderer interface {
	// RenderHTTP генерирует HTTP обработчики для контракта.
	RenderHTTP() error

	// RenderServer генерирует серверную обертку для контракта.
	RenderServer() error

	// RenderExchange генерирует структуры обмена данными (request/response).
	RenderExchange() error

	// RenderMiddleware генерирует типы middleware.
	RenderMiddleware() error

	// RenderTrace генерирует middleware для трейсинга.
	RenderTrace() error

	// RenderMetrics генерирует middleware для метрик.
	RenderMetrics() error

	// RenderLogger генерирует middleware для логирования.
	RenderLogger() error

	// RenderJsonRPC генерирует JSON-RPC обработчики.
	RenderJsonRPC() error

	// RenderREST генерирует REST обработчики.
	RenderREST() error

	// Транспортные файлы (генерируются один раз для всех контрактов)
	RenderTransportHTTP() error
	RenderTransportContext() error
	RenderTransportLogger() error
	RenderTransportFiber() error
	RenderTransportHeader() error
	RenderTransportErrors() error
	RenderTransportServer() error
	RenderTransportOptions() error
	RenderTransportMetrics() error
	RenderTransportVersion() error
	RenderTransportJsonRPC() error
}
