// Copyright (c) 2020 Khramtsov Aleksei (seniorGolang@gmail.com).
// This file is subject to the terms and conditions defined in file 'LICENSE', which is part of this project source code.
package utils

import (
	"testing"

	"tgp/internal/parser"
)

func TestValidateProject(t *testing.T) {

	tests := []struct {
		name    string
		project *parser.Project
		wantErr bool
	}{
		{
			name:    "nil project",
			project: nil,
			wantErr: true,
		},
		{
			name: "empty module path",
			project: &parser.Project{
				ModulePath: "",
			},
			wantErr: true,
		},
		{
			name: "valid project",
			project: &parser.Project{
				ModulePath: "github.com/example/project",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProject(tt.project)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContractID(t *testing.T) {

	tests := []struct {
		name       string
		contractID string
		wantErr    bool
	}{
		{
			name:       "empty contractID",
			contractID: "",
			wantErr:    true,
		},
		{
			name:       "valid contractID",
			contractID: "test-contract",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContractID(tt.contractID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContractID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOutDir(t *testing.T) {

	tests := []struct {
		name    string
		outDir  string
		wantErr bool
	}{
		{
			name:    "empty outDir",
			outDir:  "",
			wantErr: true,
		},
		{
			name:    "valid outDir",
			outDir:  "/tmp/output",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutDir(tt.outDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOutDir() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFindContract(t *testing.T) {

	project := &parser.Project{
		Contracts: []*parser.Contract{
			{ID: "contract1", Name: "Contract1"},
			{ID: "contract2", Name: "Contract2"},
		},
	}

	tests := []struct {
		name       string
		project    *parser.Project
		contractID string
		wantErr    bool
		wantName   string
	}{
		{
			name:       "nil project",
			project:    nil,
			contractID: "contract1",
			wantErr:    true,
		},
		{
			name:       "empty contractID",
			project:    project,
			contractID: "",
			wantErr:    true,
		},
		{
			name:       "contract not found",
			project:    project,
			contractID: "contract3",
			wantErr:    true,
		},
		{
			name:       "contract found",
			project:    project,
			contractID: "contract1",
			wantErr:    false,
			wantName:   "Contract1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contract, err := FindContract(tt.project, tt.contractID)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindContract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && contract.Name != tt.wantName {
				t.Errorf("FindContract() contract.Name = %v, want %v", contract.Name, tt.wantName)
			}
		})
	}
}
