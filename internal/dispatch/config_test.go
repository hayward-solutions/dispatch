package dispatch

import (
	"testing"
)

func TestParse(t *testing.T) {
	yaml := `
mode: terraform
path_to_tfvars: terraform/variables.tf
ignore_inputs:
  - assume_role_arns
  - region
flow:
  - step: Regions
    inputs:
      - additional_regions
  - step: GitHub Repositories
    description: Allow GitHub repos access with OIDC.
    inputs:
      - terraform_repositories
      - ecr_repositories
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Mode != "terraform" {
		t.Errorf("mode = %q, want terraform", cfg.Mode)
	}
	if cfg.VariablesPath != "terraform/variables.tf" {
		t.Errorf("variables_path = %q, want terraform/variables.tf", cfg.VariablesPath)
	}
	if len(cfg.IgnoreInputs) != 2 {
		t.Errorf("ignore_inputs len = %d, want 2", len(cfg.IgnoreInputs))
	}
	if len(cfg.Flow) != 2 {
		t.Errorf("flow len = %d, want 2", len(cfg.Flow))
	}
	if cfg.Flow[0].Name != "Regions" {
		t.Errorf("flow[0].name = %q, want Regions", cfg.Flow[0].Name)
	}
	if cfg.Flow[1].Description != "Allow GitHub repos access with OIDC." {
		t.Errorf("flow[1].description = %q", cfg.Flow[1].Description)
	}
	if len(cfg.Flow[1].Inputs) != 2 {
		t.Errorf("flow[1].inputs len = %d, want 2", len(cfg.Flow[1].Inputs))
	}
}

func TestParseValidation(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"missing mode", `path_to_tfvars: x
flow:
  - step: A
    inputs: [a]`},
		{"unsupported mode", `mode: ansible
path_to_tfvars: x
flow:
  - step: A
    inputs: [a]`},
		{"missing path", `mode: terraform
flow:
  - step: A
    inputs: [a]`},
		{"empty flow", `mode: terraform
path_to_tfvars: x
flow: []`},
		{"step missing name", `mode: terraform
path_to_tfvars: x
flow:
  - inputs: [a]`},
		{"step missing inputs", `mode: terraform
path_to_tfvars: x
flow:
  - step: A`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestIgnoreSet(t *testing.T) {
	cfg := &Config{IgnoreInputs: []string{"a", "b", "c"}}
	s := cfg.IgnoreSet()
	if !s["a"] || !s["b"] || !s["c"] {
		t.Error("expected all ignored inputs in set")
	}
	if s["d"] {
		t.Error("unexpected key in set")
	}
}
