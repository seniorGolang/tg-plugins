// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package renderer

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	. "github.com/dave/jennifer/jen" // nolint:staticcheck

	"tgp/plugins/server/core"
	"tgp/plugins/server/renderer/types"
)

// RenderJsonRPC генерирует JSON-RPC обработчики.
func (r *contractRenderer) RenderJsonRPC() error {

	if err := r.pkgCopyTo("context", r.outDir); err != nil {
		return fmt.Errorf("copy context package: %w", err)
	}

	srcFile := NewSrcFile(filepath.Base(r.outDir))
	srcFile.PackageComment(DoNotEdit)

	jsonPkg := r.project.Annotations.Value(TagPackageJSON, PackageStdJSON)
	srcFile.ImportName(PackageFiber, "fiber")
	srcFile.ImportName(PackageErrors, "errors")
	srcFile.ImportName(PackageSlog, "slog")
	srcFile.ImportName(PackageSync, "sync")
	srcFile.ImportName(PackageBytes, "bytes")
	srcFile.ImportName(PackageTime, "time")
	srcFile.ImportName(r.contract.PkgPath, filepath.Base(r.contract.PkgPath))
	srcFile.ImportName(jsonPkg, "json")
	srcFile.ImportName(PackageReflect, "reflect")
	srcFile.ImportName(fmt.Sprintf("%s/context", r.pkgPath(r.outDir)), "context")

	typeGen := types.NewGenerator(r.project, &srcFile)

	for _, method := range r.contract.Methods {
		if !r.methodIsJsonRPC(method) {
			continue
		}
		methodName := strings.ToLower(method.Name)
		srcFile.Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
			Id("serve" + method.Name).
			Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx")).
			Params(Err().Error()).
			Block(
				Return().Id("http").Dot("_serveMethod").Call(Id(VarNameFtx), Lit(methodName), Id("http").Dot(toLowerCamel(method.Name))),
			)
		srcFile.Add(r.rpcMethodFuncWithFiberCtx(typeGen, method, jsonPkg))
		srcFile.Add(r.rpcMethodFuncWithContext(typeGen, method, jsonPkg))
	}
	srcFile.Add(r.serveMethodFunc(jsonPkg))
	srcFile.Add(r.serviceBatchFunc(jsonPkg))
	srcFile.Add(r.serviceServeBatchFunc(jsonPkg))
	srcFile.Add(r.serviceSingleBatchFunc(typeGen))

	return srcFile.Save(path.Join(r.outDir, strings.ToLower(r.contract.Name)+"-jsonrpc.go"))
}

// rpcMethodFuncWithFiberCtx генерирует функцию обработки JSON-RPC метода с Fiber контекстом.
func (r *contractRenderer) rpcMethodFuncWithFiberCtx(typeGen *types.Generator, method *core.Method, jsonPkg string) Code {

	return Func().Params(Id("http").Op("*").Id("http"+r.contract.Name)).
		Id(toLowerCamel(method.Name)).
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx"), Id("requestBase").Id("baseJsonRPC")).
		Params(Id("responseBase").Op("*").Id("baseJsonRPC")).
		BlockFunc(func(bg *Group) {
			bg.Line()
			bg.Var().Err().Error()
			bg.Var().Id("request").Id(requestStructName(r.contract.Name, method.Name))
			bg.Var().Id("response").Id(responseStructName(r.contract.Name, method.Name))
			bg.Line()
			bg.Id("methodCtx").Op(":=").Id(VarNameFtx).Dot("UserContext").Call()
			bg.If(Id("methodCtx").Dot("Err").Call().Op("!=").Nil()).Block(
				Return(Id("makeErrorResponseJsonRPC").Call(Id("requestBase").Dot("ID"), Id("invalidRequestError"), Lit("request context cancelled"), Nil())),
			)
			bg.Line()
			bg.If(Id("requestBase").Dot("Params").Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.Id("dec").Op(":=").Qual(jsonPkg, "NewDecoder").Call(Qual(PackageBytes, "NewReader").Call(Id("requestBase").Dot("Params")))
				ig.Id("dec").Dot("DisallowUnknownFields").Call()
				ig.If(Err().Op("=").Id("dec").Dot("Decode").Call(Op("&").Id("request")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(eg *Group) {
					eg.Return(Id("makeErrorResponseJsonRPC").Call(Id("requestBase").Dot("ID"), Id("invalidParamsError"), Lit("invalid params: ").Op("+").Err().Dot("Error").Call(), Nil()))
				})
			})
			bg.Line()
			bg.ListFunc(func(lg *Group) {
				for _, ret := range resultsWithoutError(method) {
					lg.Id("response").Dot(toCamel(ret.Name))
				}
				lg.Err()
			}).Op("=").Id("http").Dot("svc").Dot(method.Name).CallFunc(func(cg *Group) {
				cg.Id("methodCtx")
				for _, arg := range argsWithoutContext(method) {
					argCode := Id("request").Dot(toCamel(arg.Name))
					if arg.IsEllipsis {
						argCode.Op("...")
					}
					cg.Add(argCode)
				}
			})
			bg.If(Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.If(Id("http").Dot("errorHandler").Op("!=").Nil()).Block(
					Err().Op("=").Id("http").Dot("errorHandler").Call(Err()),
				)
				ig.Id("code").Op(":=").Id("internalError")
				ig.If(List(Id("errCoder"), Id("ok")).Op(":=").Err().Op(".").Call(Id("withErrorCode")).Op(";").Id("ok")).Block(
					Id("code").Op("=").Id("errCoder").Dot("Code").Call(),
				)
				ig.Return(Id("makeErrorResponseJsonRPC").Call(Id("requestBase").Dot("ID"), Id("code"), Id("sanitizeErrorMessage").Call(Err()), Nil()))
			})
			bg.Id("responseBase").Op("=").Op("&").Id("baseJsonRPC").Values(Dict{
				Id("ID"):      Id("requestBase").Dot("ID"),
				Id("Version"): Id("Version"),
			})
			resp := Id("response")
			if len(resultsWithoutError(method)) == 1 && method.Annotations.Contains(TagHttpEnableInlineSingle) {
				resp = Id("response").Dot(toCamel(resultsWithoutError(method)[0].Name))
			}
			bg.If(List(Id("responseBase").Dot("Result"), Err()).Op("=").Qual(jsonPkg, "Marshal").Call(resp).Op(";").Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.Return(Id("makeErrorResponseJsonRPC").Call(Id("requestBase").Dot("ID"), Id("parseError"), Lit("response body could not be encoded: ").Op("+").Err().Dot("Error").Call(), Nil()))
			})
			bg.Line()
			bg.Return()
		})
}

// rpcMethodFuncWithContext генерирует функцию обработки JSON-RPC метода с контекстом.
func (r *contractRenderer) rpcMethodFuncWithContext(typeGen *types.Generator, method *core.Method, jsonPkg string) Code {

	return Func().Params(Id("http").Op("*").Id("http"+r.contract.Name)).
		Id(toLowerCamel(method.Name)+"WithContext").
		Params(Id(VarNameCtx).Qual(fmt.Sprintf("%s/context", r.pkgPath(r.outDir)), "Context"), Id("requestBase").Id("baseJsonRPC")).
		Params(Id("responseBase").Op("*").Id("baseJsonRPC")).
		BlockFunc(func(bg *Group) {
			bg.Line()
			bg.Var().Err().Error()
			bg.Var().Id("request").Id(requestStructName(r.contract.Name, method.Name))
			bg.Var().Id("response").Id(responseStructName(r.contract.Name, method.Name))
			bg.Line()
			bg.If(Id(VarNameCtx).Dot("Err").Call().Op("!=").Nil()).Block(
				Return(Id("makeErrorResponseJsonRPC").Call(Id("requestBase").Dot("ID"), Id("invalidRequestError"), Lit("request context cancelled"), Nil())),
			)
			bg.Line()
			bg.If(Id("requestBase").Dot("Params").Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.Id("dec").Op(":=").Qual(jsonPkg, "NewDecoder").Call(Qual(PackageBytes, "NewReader").Call(Id("requestBase").Dot("Params")))
				ig.Id("dec").Dot("DisallowUnknownFields").Call()
				ig.If(Err().Op("=").Id("dec").Dot("Decode").Call(Op("&").Id("request")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(eg *Group) {
					eg.Return(Id("makeErrorResponseJsonRPC").Call(Id("requestBase").Dot("ID"), Id("invalidParamsError"), Lit("invalid params: ").Op("+").Err().Dot("Error").Call(), Nil()))
				})
			})
			bg.Line()
			bg.ListFunc(func(lg *Group) {
				for _, ret := range resultsWithoutError(method) {
					lg.Id("response").Dot(toCamel(ret.Name))
				}
				lg.Err()
			}).Op("=").Id("http").Dot("svc").Dot(method.Name).CallFunc(func(cg *Group) {
				cg.Id(VarNameCtx)
				for _, arg := range argsWithoutContext(method) {
					argCode := Id("request").Dot(toCamel(arg.Name))
					if arg.IsEllipsis {
						argCode.Op("...")
					}
					cg.Add(argCode)
				}
			})
			bg.If(Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.If(Id("http").Dot("errorHandler").Op("!=").Nil()).Block(
					Err().Op("=").Id("http").Dot("errorHandler").Call(Err()),
				)
				ig.Id("code").Op(":=").Id("internalError")
				ig.If(List(Id("errCoder"), Id("ok")).Op(":=").Err().Op(".").Call(Id("withErrorCode")).Op(";").Id("ok")).Block(
					Id("code").Op("=").Id("errCoder").Dot("Code").Call(),
				)
				ig.Return(Id("makeErrorResponseJsonRPC").Call(Id("requestBase").Dot("ID"), Id("code"), Id("sanitizeErrorMessage").Call(Err()), Nil()))
			})
			bg.Id("responseBase").Op("=").Op("&").Id("baseJsonRPC").Values(Dict{
				Id("ID"):      Id("requestBase").Dot("ID"),
				Id("Version"): Id("Version"),
			})
			resp := Id("response")
			if len(resultsWithoutError(method)) == 1 && method.Annotations.Contains(TagHttpEnableInlineSingle) {
				resp = Id("response").Dot(toCamel(resultsWithoutError(method)[0].Name))
			}
			bg.If(List(Id("responseBase").Dot("Result"), Err()).Op("=").Qual(jsonPkg, "Marshal").Call(resp).Op(";").Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.Return(Id("makeErrorResponseJsonRPC").Call(Id("requestBase").Dot("ID"), Id("parseError"), Lit("response body could not be encoded: ").Op("+").Err().Dot("Error").Call(), Nil()))
			})
			bg.Line()
			bg.Return()
		})
}

// serveMethodFunc генерирует общую функцию обработки метода.
func (r *contractRenderer) serveMethodFunc(jsonPkg string) Code {

	return Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
		Id("_serveMethod").
		ParamsFunc(func(pg *Group) {
			pg.Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx")
			pg.Id("methodName").String()
			pg.Id("methodHandler").Id("methodJsonRPCWithFiber")
		}).
		Params(Err().Error()).
		BlockFunc(func(bg *Group) {
			bg.Line()
			bg.Id("methodHTTP").Op(":=").Id(VarNameFtx).Dot("Method").Call()
			bg.If(Id("methodHTTP").Op("!=").Qual(PackageFiber, "MethodPost")).BlockFunc(func(ig *Group) {
				ig.Id(VarNameFtx).Dot("Response").Call().Dot("SetStatusCode").Call(Qual(PackageFiber, "StatusMethodNotAllowed"))
				ig.If(List(Id("_"), Err()).Op("=").Id(VarNameFtx).Dot("WriteString").Call(Lit("only POST method supported")).Op(";").Err().Op("!=").Nil()).Block(
					Return(),
				)
			})
			bg.Var().Id("request").Id("baseJsonRPC")
			bg.Var().Id("response").Op("*").Id("baseJsonRPC")
			bg.If(Err().Op("=").Qual(jsonPkg, "Unmarshal").Call(Id(VarNameFtx).Dot("Body").Call(), Op("&").Id("request")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.Return().Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("request body could not be decoded: ").Op("+").Err().Dot("Error").Call())
			})
			bg.If(Err().Op("=").Id("validateJsonRPCRequest").Call(Id("request")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(ig *Group) {
				ig.Return().Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("invalid JSON-RPC request: ").Op("+").Err().Dot("Error").Call())
			})
			bg.Id("methodNameOrigin").Op(":=").Id("request").Dot("Method")
			bg.Id("method").Op(":=").Id("toLowercaseMethod").Call(Id("request").Dot("Method"))

			bg.If(Id("method").Op("!=").Lit("").Op("&&").Id("method").Op("!=").Id("methodName")).BlockFunc(func(ig *Group) {
				ig.Return().Id("sendResponse").Call(Id(VarNameFtx), Id("makeErrorResponseJsonRPC").Call(Id("request").Dot("ID"), Id("methodNotFoundError"), Lit("invalid method ").Op("+").Id("methodNameOrigin"), Nil()))
			})
			bg.Id("response").Op("=").Id("methodHandler").Call(Id(VarNameFtx), Id("request"))
			bg.If(Id("response").Op("!=").Nil()).Block(
				Return().Id("sendResponse").Call(Id(VarNameFtx), Id("response")),
			)
			bg.Return()
		})
}

// serviceBatchFunc генерирует функцию обработки batch запросов.
func (r *contractRenderer) serviceBatchFunc(jsonPkg string) Code {

	return Func().Params(Id("http").Op("*").Id("http"+r.contract.Name)).
		Id("doBatch").
		Params(Id(VarNameFtx).Op("*").Qual(PackageFiber, "Ctx"), Id("requests").Op("[]").Id("baseJsonRPC")).
		Params(Id("responses").Op("[]").Op("*").Id("baseJsonRPC")).
		Block(
			Return(Id("http").Dot("srv").Dot("doBatch").Call(Id(VarNameFtx), Id("requests"))),
		)
}

// serviceServeBatchFunc генерирует функцию обработки batch запросов.
func (r *contractRenderer) serviceServeBatchFunc(jsonPkg string) Code {

	return Func().Params(Id("http").Op("*").Id("http" + r.contract.Name)).
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
				Return(Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("request body could not be decoded: empty body"))),
			)
			bg.Id("decoder").Op(":=").Qual(jsonPkg, "NewDecoder").Call(Qual(PackageBytes, "NewReader").Call(Id("body")))
			bg.Id("decoder").Dot("DisallowUnknownFields").Call()
			bg.Var().Id("token").Interface()
			bg.List(Id("token"), Err()).Op("=").Id("decoder").Dot("Token").Call()
			bg.If(Err().Op("!=").Nil()).Block(
				Return(Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("request body could not be decoded: ").Op("+").Err().Dot("Error").Call())),
			)
			bg.If(Id("token").Op("==").Qual(jsonPkg, "Delim").Call(Lit('['))).BlockFunc(func(ig *Group) {
				ig.For(Id("decoder").Dot("More").Call()).BlockFunc(func(fg *Group) {
					fg.Var().Id("request").Id("baseJsonRPC")
					fg.If(Err().Op("=").Id("decoder").Dot("Decode").Call(Op("&").Id("request")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(dg *Group) {
						dg.Return(Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("request body could not be decoded: ").Op("+").Err().Dot("Error").Call()))
					})
					fg.Id("requests").Op("=").Append(Id("requests"), Id("request"))
				})
			}).Else().BlockFunc(func(ig *Group) {
				ig.Var().Id("request").Id("baseJsonRPC")
				ig.If(Err().Op("=").Qual(jsonPkg, "Unmarshal").Call(Id("body"), Op("&").Id("request")).Op(";").Err().Op("!=").Nil()).BlockFunc(func(ug *Group) {
					ug.Return(Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("request body could not be decoded: ").Op("+").Err().Dot("Error").Call()))
				})
				ig.Id("single").Op("=").True()
				ig.Id("requests").Op("=").Append(Id("requests"), Id("request"))
			})
			bg.If(Len(Id("requests")).Op("==").Lit(0)).Block(
				Return(Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("empty batch request"))),
			)
			bg.If(Len(Id("requests")).Op(">").Id("http").Dot("srv").Dot("maxBatchSize")).Block(
				Return(Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("batch size exceeded"))),
			)
			bg.If(Id("single")).BlockFunc(func(ig *Group) {
				ig.If(Err().Op("=").Id("validateJsonRPCRequest").Call(Id("requests").Op("[").Lit(0).Op("]")).Op(";").Err().Op("!=").Nil()).Block(
					Return(Id("sendHTTPError").Call(Id(VarNameFtx), Qual(PackageFiber, "StatusBadRequest"), Lit("invalid JSON-RPC request: ").Op("+").Err().Dot("Error").Call())),
				)
				ig.Return(Id("sendResponse").Call(Id(VarNameFtx), Id("http").Dot("srv").Dot("doSingleBatch").
					Call(Id(VarNameFtx).Dot("UserContext").Call(), Id("requests").Op("[").Lit(0).Op("]")),
				))
			})
			bg.Return(Id("sendResponse").Call(Id(VarNameFtx), Id("http").Dot("doBatch").
				Call(Id(VarNameFtx), Id("requests")),
			))
		})
}

// serviceSingleBatchFunc генерирует функцию обработки одиночного batch запроса.
func (r *contractRenderer) serviceSingleBatchFunc(typeGen *types.Generator) Code {

	return Func().Params(Id("http").Op("*").Id("http"+r.contract.Name)).
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
			bg.Id("method").Op(":=").Id("toLowercaseMethod").Call(Id("request").Dot("Method"))
			bg.Switch(Id("method")).BlockFunc(func(sg *Group) {
				for _, method := range r.contract.Methods {
					if !r.methodIsJsonRPC(method) {
						continue
					}
					methodNameLC := strings.ToLower(method.Name)
					sg.Case(Lit(methodNameLC)).Block(
						Return(Id("http").Dot(toLowerCamel(method.Name)+"WithContext").Call(Id(VarNameCtx), Id("request"))),
					)
				}
				sg.Default().BlockFunc(func(dg *Group) {
					dg.Return(Id("makeErrorResponseJsonRPC").Call(Id("request").Dot("ID"), Id("methodNotFoundError"), Lit("invalid method '").Op("+").Id("methodNameOrigin").Op("+").Lit("'"), Nil()))
				})
			})
		})
}
