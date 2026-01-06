// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package utils

import (
	"testing"

	"tgp/internal/parser"
)

func TestIsContextFirst(t *testing.T) {

	tests := []struct {
		name string
		vars []*parser.Variable
		want bool
	}{
		{
			name: "empty vars",
			vars: []*parser.Variable{},
			want: false,
		},
		{
			name: "context first",
			vars: []*parser.Variable{
				{TypeID: "context:Context"},
				{TypeID: "string"},
			},
			want: true,
		},
		{
			name: "context not first",
			vars: []*parser.Variable{
				{TypeID: "string"},
				{TypeID: "context:Context"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsContextFirst(tt.vars); got != tt.want {
				t.Errorf("IsContextFirst() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsErrorLast(t *testing.T) {

	tests := []struct {
		name string
		vars []*parser.Variable
		want bool
	}{
		{
			name: "empty vars",
			vars: []*parser.Variable{},
			want: false,
		},
		{
			name: "error last",
			vars: []*parser.Variable{
				{TypeID: "string"},
				{TypeID: "error"},
			},
			want: true,
		},
		{
			name: "error not last",
			vars: []*parser.Variable{
				{TypeID: "error"},
				{TypeID: "string"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsErrorLast(tt.vars); got != tt.want {
				t.Errorf("IsErrorLast() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArgsWithoutContext(t *testing.T) {

	tests := []struct {
		name   string
		method *parser.Method
		want   int
	}{
		{
			name: "with context",
			method: &parser.Method{
				Args: []*parser.Variable{
					{TypeID: "context:Context"},
					{TypeID: "string"},
				},
			},
			want: 1,
		},
		{
			name: "without context",
			method: &parser.Method{
				Args: []*parser.Variable{
					{TypeID: "string"},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ArgsWithoutContext(tt.method)
			if len(got) != tt.want {
				t.Errorf("ArgsWithoutContext() len = %v, want %v", len(got), tt.want)
			}
		})
	}
}

func TestResultsWithoutError(t *testing.T) {

	tests := []struct {
		name   string
		method *parser.Method
		want   int
	}{
		{
			name: "with error",
			method: &parser.Method{
				Results: []*parser.Variable{
					{TypeID: "string"},
					{TypeID: "error"},
				},
			},
			want: 1,
		},
		{
			name: "without error",
			method: &parser.Method{
				Results: []*parser.Variable{
					{TypeID: "string"},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResultsWithoutError(tt.method)
			if len(got) != tt.want {
				t.Errorf("ResultsWithoutError() len = %v, want %v", len(got), tt.want)
			}
		})
	}
}

func TestRequestStructName(t *testing.T) {

	if got := RequestStructName("Contract", "Method"); got != "requestContractMethod" {
		t.Errorf("RequestStructName() = %v, want requestContractMethod", got)
	}
}

func TestResponseStructName(t *testing.T) {

	if got := ResponseStructName("Contract", "Method"); got != "responseContractMethod" {
		t.Errorf("ResponseStructName() = %v, want responseContractMethod", got)
	}
}
