// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/plugins/server/core"
)

// jsonBufferPools генерирует пулы буферов для JSON-RPC.
func (r *transportRenderer) jsonBufferPools() Code {

	return Var().DefsFunc(func(dg *Group) {
		dg.Id("bufferPool").Op("=").Qual(PackageSync, "Pool").Values(Dict{
			Id("New"): Func().Params().Interface().Block(
				Return(Qual(PackageBytes, "NewBuffer").Call(Make(Index().Byte(), Lit(0), Lit(4096)))),
			),
		})
	})
}

// jsonRPCMethodMap генерирует карту методов JSON-RPC.
func (r *transportRenderer) jsonRPCMethodMap() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("jsonRPCMethodMap").
		Params().
		Params(Map(String()).Id("methodJsonRPC")).
		BlockFunc(func(bg *Group) {
			bg.Return(Map(String()).Id("methodJsonRPC").Values(DictFunc(func(dict Dict) {
				for _, contract := range r.project.Contracts {
					if !contract.Annotations.Contains(TagServerJsonRPC) {
						continue
					}
					for _, method := range contract.Methods {
						if !r.methodIsJsonRPCForContract(contract, method) {
							continue
						}
						contractLCName := strings.ToLower(contract.Name)
						methodLCName := strings.ToLower(method.Name)
						methodKey := contractLCName + "." + methodLCName
						dict[Lit(methodKey)] = Func().
							Params(Id(VarNameCtx).Qual(fmt.Sprintf("%s/context", r.pkgPath(r.outDir)), "Context"), Id("requestBase").Id("baseJsonRPC")).
							Params(Id("responseBase").Op("*").Id("baseJsonRPC")).
							Block(
								Return(Id("srv").Dot("http"+contract.Name).Dot(toLowerCamel(method.Name)+"WithContext").Call(Id(VarNameCtx), Id("requestBase"))),
							)
					}
				}
			})))
		})
}

// singleBatchFunc генерирует функцию doSingleBatch.
func (r *transportRenderer) singleBatchFunc() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("doSingleBatch").
		Params(Id(VarNameCtx).Qual(fmt.Sprintf("%s/context", r.pkgPath(r.outDir)), "Context"), Id("request").Id("baseJsonRPC")).
		Params(Id("response").Op("*").Id("baseJsonRPC")).
		BlockFunc(func(bg *Group) {
			bg.Line()
			bg.Var().Err().Error()
			bg.If(Err().Op("=").Id("validateJsonRPCRequest").Call(Id("request")).Op(";").Err().Op("!=").Nil()).Block(
				Return(Id("makeErrorResponseJsonRPC").Call(Id("request").Dot("ID"), Id("invalidRequestError"), Lit("invalid JSON-RPC request: ").Op("+").Err().Dot("Error").Call(), Nil())),
			)
			bg.Id("methodNameOrigin").Op(":=").Id("request").Dot("Method")
			bg.Id("method").Op(":=").Id("toLowercaseMethod").Call(Id("methodNameOrigin"))
			bg.Id("methodMap").Op(":=").Id("srv").Dot("jsonRPCMethodMap").Call()
			bg.Id("handler").Op(",").Id("ok").Op(":=").Id("methodMap").Index(Id("method"))
			bg.If(Op("!").Id("ok")).Block(
				Return(Id("makeErrorResponseJsonRPC").Call(Id("request").Dot("ID"), Id("methodNotFoundError"), Lit("invalid method '").Op("+").Id("methodNameOrigin").Op("+").Lit("'"), Nil())),
			)
			bg.Return(Id("handler").Call(Id(VarNameCtx), Id("request")))
		})
}

// batchFunc генерирует функцию doBatch.
func (r *transportRenderer) batchFunc() Code {

	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("doBatch").
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx"), Id("requests").Op("[]").Id("baseJsonRPC")).
		Params(Id("responses").Op("[]").Op("*").Id("baseJsonRPC")).
		BlockFunc(func(bg *Group) {
			bg.Line()
			// Извлекаем контекст и конфигурацию до запуска горутин
			bg.Id("userCtx").Op(":=").Id(VarNameFtx).Dot("UserContext").Call()
			bg.Id("batchTimeout").Op(":=").Id(VarNameFtx).Dot("App").Call().Dot("Config").Call().Dot("WriteTimeout")
			bg.Var().Id("batchCtx").Qual(fmt.Sprintf("%s/context", r.pkgPath(r.outDir)), "Context")
			bg.Var().Id("cancel").Qual(fmt.Sprintf("%s/context", r.pkgPath(r.outDir)), "CancelFunc")
			bg.If(Id("batchTimeout").Op(">").Lit(0)).Block(
				List(Id("batchCtx"), Id("cancel")).Op("=").Qual(fmt.Sprintf("%s/context", r.pkgPath(r.outDir)), "WithTimeout").Call(Id("userCtx"), Id("batchTimeout")),
				Defer().Id("cancel").Call(),
			).Else().Block(
				Id("batchCtx").Op("=").Id("userCtx"),
			)
			bg.If(Qual(PackageStrings, "EqualFold").Call(Id(VarNameFtx).Dot("Get").Call(Id("syncHeader")), Lit("true"))).Block(
				Id("syncResponses").Op(":=").Make(Index().Op("*").Id("baseJsonRPC"), Lit(0), Len(Id("requests"))),
				For(List(Id("_"), Id("request")).Op(":=").Range().Id("requests")).Block(
					Id("response").Op(":=").Id("srv").Dot("doSingleBatch").Call(Id("batchCtx"), Id("request")),
					If(Id("request").Dot("ID").Op("!=").Nil()).Block(
						Id("syncResponses").Op("=").Append(Id("syncResponses"), Id("response")),
					),
				),
				Return(Id("syncResponses")),
			)
			bg.Var().Id("wg").Qual(PackageSync, "WaitGroup")
			bg.Id("batchSize").Op(":=").Id("srv").Dot("maxParallelBatch")
			bg.If(Len(Id("requests")).Op("<").Id("batchSize")).Block(
				Id("batchSize").Op("=").Len(Id("requests")),
			)
			bg.Id("callCh").Op(":=").Make(Chan().Id("baseJsonRPC"), Id("batchSize"))
			bg.Line()
			// Подсчитываем ожидаемое количество ответов для оптимального размера буфера канала
			bg.Id("expectedCount").Op(":=").Lit(0)
			bg.For(List(Id("_"), Id("req")).Op(":=").Range().Id("requests")).Block(
				If(Id("req").Dot("ID").Op("!=").Nil()).Block(
					Id("expectedCount").Op("++"),
				),
			)
			bg.Id("resultCh").Op(":=").Make(Chan().Op("*").Id("baseJsonRPC"), Id("expectedCount"))
			bg.Line()
			bg.For(Id("i").Op(":=").Lit(0).Op(";").Id("i").Op("<").Id("batchSize").Op(";").Id("i").Op("++")).Block(
				Id("wg").Dot("Add").Call(Lit(1)),
				Go().Func().Params().Block(
					Defer().Id("wg").Dot("Done").Call(),
					For(Id("request").Op(":=").Range().Id("callCh")).BlockFunc(func(fg *Group) {
						fg.Select().Block(
							Case(Op("<-").Id("batchCtx").Dot("Done").Call()).Block(
								Return(),
							),
							Default().BlockFunc(func(dg *Group) {
								dg.Id("response").Op(":=").Id("srv").Dot("doSingleBatch").Call(Id("batchCtx"), Id("request"))
								dg.If(Id("request").Dot("ID").Op("!=").Nil()).Block(
									Select().Block(
										Case(Id("resultCh").Op("<-").Id("response")).Block(),
										Case(Op("<-").Id("batchCtx").Dot("Done").Call()).Block(
											Return(),
										),
									),
								)
							}),
						)
					}),
				).Call(),
			)
			// Проверка контекста при отправке запросов
			bg.For(List(Id("idx")).Op(":=").Range().Id("requests")).Block(
				Select().Block(
					Case(Id("callCh").Op("<-").Id("requests").Index(Id("idx"))).Block(),
					Case(Op("<-").Id("batchCtx").Dot("Done").Call()).Block(
						Close(Id("callCh")),
						Return(),
					),
				),
			)
			bg.Close(Id("callCh"))
			bg.Line()
			bg.Id("responses").Op("=").Make(Index().Op("*").Id("baseJsonRPC"), Lit(0), Id("expectedCount"))
			bg.Id("received").Op(":=").Lit(0)
			bg.If(Id("batchTimeout").Op(">").Lit(0)).Block(
				For(Id("received").Op("<").Id("expectedCount")).Block(
					Select().Block(
						Case(List(Id("resp"), Id("ok")).Op(":=").Op("<-").Id("resultCh")).Block(
							If(Op("!").Id("ok")).Block(
								Return(),
							),
							Id("responses").Op("=").Append(Id("responses"), Id("resp")),
							Id("received").Op("++"),
						),
						Case(Op("<-").Id("batchCtx").Dot("Done").Call()).Block(
							If(Id("cancel").Op("!=").Nil()).Block(
								Id("cancel").Call(),
							),
							Id("wg").Dot("Wait").Call(),
							Close(Id("resultCh")),
							Return(),
						),
					),
				),
				Id("wg").Dot("Wait").Call(),
				Close(Id("resultCh")),
			).Else().Block(
				For(Id("response").Op(":=").Range().Id("resultCh")).Block(
					Id("responses").Op("=").Append(Id("responses"), Id("response")),
				),
				Id("wg").Dot("Wait").Call(),
				Close(Id("resultCh")),
			)
			bg.Return()
		})
}

// serveBatchFunc генерирует функцию serveBatch.
func (r *transportRenderer) serveBatchFunc() Code {

	jsonPkg := r.project.Annotations.Value(TagPackageJSON, PackageStdJSON)
	return Func().Params(Id("srv").Op("*").Id("Server")).
		Id("serveBatch").
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx")).
		Params(Id("err").Error()).
		BlockFunc(func(bg *Group) {
			bg.Line()
			bg.Var().Id("single").Bool()
			bg.Var().Id("requests").Op("[]").Id("baseJsonRPC")
			bg.Id("methodHTTP").Op(":=").Id(VarNameFtx).Dot("Method").Call()
			bg.If(Id("methodHTTP").Op("!=").Qual(PackageFiber, "MethodPost")).BlockFunc(func(ig *Group) {
				ig.Id(VarNameFtx).Dot("Response").Call().Dot("SetStatusCode").Call(Qual(PackageFiber, "StatusMethodNotAllowed"))
				ig.If(List(Id("_"), Err()).Op("=").Id(VarNameFtx).Dot("WriteString").Call(Lit("only POST method supported")).Op(";").Err().Op("!=").Nil()).Block(
					Return(),
				)
				ig.Return()
			})
			bg.Id("body").Op(":=").Qual(PackageBytes, "TrimSpace").Call(Id(VarNameFtx).Dot("Body").Call())
			bg.If(Len(Id("body")).Op("==").Lit(0)).Block(
				Return(Id("sendResponse").Call(Id(VarNameFtx), Id("makeErrorResponseJsonRPC").Call(Nil(), Id("parseError"), Lit("request body could not be decoded: empty body"), Nil()))),
			)
			bg.Id("decoder").Op(":=").Qual(jsonPkg, "NewDecoder").Call(Qual(PackageBytes, "NewReader").Call(Id("body")))
			bg.Id("decoder").Dot("DisallowUnknownFields").Call()
			bg.Var().Id("token").Interface()
			bg.List(Id("token"), Err()).Op("=").Id("decoder").Dot("Token").Call()
			bg.If(Err().Op("!=").Nil()).Block(
				Return(Id("sendResponse").Call(Id(VarNameFtx), Id("makeErrorResponseJsonRPC").Call(Nil(), Id("parseError"), Lit("request body could not be decoded: ").Op("+").Err().Dot("Error").Call(), Nil()))),
			)
			bg.If(Id("token").Op("==").Qual(jsonPkg, "Delim").Call(Lit('['))).BlockFunc(func(ig *Group) {
				// Проверка на пустой массив
				ig.If(Op("!").Id("decoder").Dot("More").Call()).Block(
					Return(Id("sendResponse").Call(Id(VarNameFtx), Id("makeErrorResponseJsonRPC").Call(Nil(), Id("invalidRequestError"), Lit("empty batch request"), Nil()))),
				)
				ig.For(Id("decoder").Dot("More").Call()).BlockFunc(func(fg *Group) {
					fg.Var().Id("request").Id("baseJsonRPC")
					fg.If(Err().Op("=").Id("decoder").Dot("Decode").Call(Op("&").Id("request")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(dg *Group) {
						dg.Return(Id("sendResponse").Call(Id(VarNameFtx), Id("makeErrorResponseJsonRPC").Call(Nil(), Id("parseError"), Lit("request body could not be decoded: ").Op("+").Err().Dot("Error").Call(), Nil())))
					})
					fg.Id("requests").Op("=").Append(Id("requests"), Id("request"))
				})
			}).Else().BlockFunc(func(ig *Group) {
				ig.Var().Id("request").Id("baseJsonRPC")
				ig.If(Err().Op("=").Qual(jsonPkg, "Unmarshal").Call(Id("body"), Op("&").Id("request")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(ug *Group) {
					ug.Return(Id("sendResponse").Call(Id(VarNameFtx), Id("makeErrorResponseJsonRPC").Call(Nil(), Id("parseError"), Lit("request body could not be decoded: ").Op("+").Err().Dot("Error").Call(), Nil())))
				})
				ig.Id("single").Op("=").True()
				ig.Id("requests").Op("=").Append(Id("requests"), Id("request"))
			})
			bg.If(Len(Id("requests")).Op(">").Id("srv").Dot("maxBatchSize")).Block(
				Return(Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("batch size exceeded"))),
			)
			bg.If(Id("single")).Block(
				Return(Id("sendResponse").Call(Id(VarNameFtx), Id("srv").Dot("doSingleBatch").Call(Id(VarNameFtx).Dot("UserContext").Call(), Id("requests").Op("[").Lit(0).Op("]")))),
			)
			bg.Return(Id("sendResponse").Call(Id(VarNameFtx), Id("srv").Dot("doBatch").Call(Id(VarNameFtx), Id("requests"))))
		})
}

// toLowercaseMethodFunc генерирует функцию toLowercaseMethod.
func (r *transportRenderer) toLowercaseMethodFunc() Code {

	return Func().Id("toLowercaseMethod").
		Params(Id("s").String()).
		Params(String()).
		Block(
			Return(Qual(PackageStrings, "ToLower").Call(Id("s"))),
		)
}

// sanitizeErrorMessageFunc генерирует функцию sanitizeErrorMessage.
func (r *transportRenderer) sanitizeErrorMessageFunc() Code {

	return Func().Id("sanitizeErrorMessage").
		Params(Err().Error()).
		Params(String()).
		Block(
			If(Err().Op("==").Nil()).Block(
				Return(Lit("")),
			),
			Id("message").Op(":=").Err().Dot("Error").Call(),
			If(Id("idx").Op(":=").Qual(PackageStrings, "IndexByte").Call(Id("message"), Add(Id("'\\n'"))).Op(";").Id("idx").Op(">=").Lit(0)).Block(
				Return(Id("message").Index(Op(":").Id("idx"))),
			),
			Return(Id("message")),
		)
}

// validateJsonRPCRequestFunc генерирует функцию validateJsonRPCRequest.
func (r *transportRenderer) validateJsonRPCRequestFunc() Code {

	return Func().Id("validateJsonRPCRequest").
		Params(Id("requestBase").Id("baseJsonRPC")).
		Params(Err().Error()).
		BlockFunc(func(bg *Group) {
			bg.If(Id("requestBase").Dot("Version").Op("==").Lit("")).Block(
				Return(Qual(PackageErrors, "New").Call(Lit("missing protocol version"))),
			)
			bg.If(Id("requestBase").Dot("Version").Op("!=").Id("Version")).Block(
				Return(Qual(PackageFmt, "Errorf").Call(Lit("incorrect protocol version: %s"), Id("requestBase").Dot("Version"))),
			)
			bg.Return(Nil())
		})
}

// makeErrorResponseJsonRPCFunc генерирует функцию makeErrorResponseJsonRPC.
func (r *transportRenderer) makeErrorResponseJsonRPCFunc() Code {

	return Func().Id("makeErrorResponseJsonRPC").
		Params(Id("id").Id("idJsonRPC"), Id("code").Int(), Id("msg").String(), Id("data").Interface()).
		Params(Op("*").Id("baseJsonRPC")).
		BlockFunc(func(bg *Group) {
			bg.If(Id("id").Op("==").Nil()).Block(
				Return(Nil()),
			)
			bg.Return(Op("&").Id("baseJsonRPC").Values(Dict{
				Id("ID"):      Id("id"),
				Id("Version"): Id("Version"),
				Id("Error"): Op("&").Id("errorJsonRPC").Values(Dict{
					Id("Code"):    Id("code"),
					Id("Message"): Id("msg"),
					Id("Data"):    Id("data"),
				}),
			}))
		})
}

// methodIsJsonRPCForContract проверяет, является ли метод JSON-RPC методом для указанного контракта.
func (r *transportRenderer) methodIsJsonRPCForContract(contract *core.Contract, method *core.Method) bool {

	if method == nil || method.Annotations == nil {
		return false
	}
	return contract != nil && contract.Annotations.Contains(TagServerJsonRPC) && !method.Annotations.Contains(TagMethodHTTP)
}
