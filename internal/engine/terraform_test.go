package engine

import (
	"testing"
)

// testVariablesTF mimics the vending repo's variables.tf
const testVariablesTF = `
variable "region" {
  description = "The AWS region to deploy resources in."
  type        = string
  default     = "eu-west-2"
}

variable "additional_regions" {
  description = "Additional AWS regions which users can provision resources."
  type        = list(string)
  default     = []
}

variable "ecr_repositories" {
  description = "Map of GitHub repositories to create ECR repositories and OIDC Roles for."
  type = map(object({
    github_repo = string
    github_org  = optional(string, "hayward-solutions")
  }))
  default = {}
}

variable "terraform_repositories" {
  description = "Map of GitHub repositories to create OIDC Roles for."
  type = map(object({
    github_repo = string
    github_org  = optional(string, "hayward-solutions")
  }))
  default = {}
}

variable "bucket_prefix" {
  description = "The prefix to use for S3 bucket names."
  type        = string
  default     = "hs"
}

variable "new_relic" {
  type = object({
    enabled    = optional(bool, false)
    account_id = optional(string, "7029769")
    region     = optional(string, "EU")
  })
  default = {}
}

variable "email" {
  description = "The email address associated with the AWS account."
  type        = string
  default     = "aws@hayward.solutions"
}

variable "ou_name" {
  description = "The name of the organisational unit (OU) to assign this account."
  type        = string
  default     = "Applications"
}

variable "assume_role_arns" {
  description = "The ARN of the role to assume for Terraform operations."
  type        = map(string)
  default = {
    management = "arn:aws:iam::721407894561:role/terraform-write"
  }
}
`

func TestTerraformParseVariables(t *testing.T) {
	engine := &TerraformEngine{}
	vars, err := engine.ParseVariables([]byte(testVariablesTF))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Variables are sorted by name
	byName := make(map[string]Variable)
	for _, v := range vars {
		byName[v.Name] = v
	}

	t.Run("string variable", func(t *testing.T) {
		v := byName["region"]
		if v.Type.Kind != TypeString {
			t.Errorf("type = %v, want string", v.Type.Kind)
		}
		if v.Default != "eu-west-2" {
			t.Errorf("default = %v, want eu-west-2", v.Default)
		}
		if v.Description != "The AWS region to deploy resources in." {
			t.Errorf("description = %q", v.Description)
		}
	})

	t.Run("list(string) variable", func(t *testing.T) {
		v := byName["additional_regions"]
		if v.Type.Kind != TypeList {
			t.Errorf("type = %v, want list", v.Type.Kind)
		}
		if v.Type.ElementType == nil || v.Type.ElementType.Kind != TypeString {
			t.Error("element type should be string")
		}
		items, ok := v.Default.([]any)
		if !ok {
			t.Fatalf("default type = %T, want []any", v.Default)
		}
		if len(items) != 0 {
			t.Errorf("default len = %d, want 0", len(items))
		}
	})

	t.Run("map(string) variable", func(t *testing.T) {
		v := byName["assume_role_arns"]
		if v.Type.Kind != TypeMap {
			t.Errorf("type = %v, want map", v.Type.Kind)
		}
		if v.Type.ElementType == nil || v.Type.ElementType.Kind != TypeString {
			t.Error("element type should be string")
		}
		m, ok := v.Default.(map[string]any)
		if !ok {
			t.Fatalf("default type = %T, want map[string]any", v.Default)
		}
		if m["management"] != "arn:aws:iam::721407894561:role/terraform-write" {
			t.Errorf("default[management] = %v", m["management"])
		}
	})

	t.Run("map(object) variable", func(t *testing.T) {
		v := byName["ecr_repositories"]
		if v.Type.Kind != TypeMap {
			t.Errorf("type = %v, want map", v.Type.Kind)
		}
		if v.Type.ElementType == nil || v.Type.ElementType.Kind != TypeObject {
			t.Fatal("element type should be object")
		}
		attrs := v.Type.ElementType.Attributes
		if len(attrs) != 2 {
			t.Fatalf("attributes len = %d, want 2", len(attrs))
		}
		// Attributes are sorted by name
		foundRepo := false
		foundOrg := false
		for _, attr := range attrs {
			switch attr.Name {
			case "github_repo":
				foundRepo = true
				if attr.Type.Kind != TypeString {
					t.Errorf("github_repo type = %v, want string", attr.Type.Kind)
				}
				if attr.Optional {
					t.Error("github_repo should not be optional")
				}
			case "github_org":
				foundOrg = true
				if attr.Type.Kind != TypeString {
					t.Errorf("github_org type = %v, want string", attr.Type.Kind)
				}
				if !attr.Optional {
					t.Error("github_org should be optional")
				}
			}
		}
		if !foundRepo || !foundOrg {
			t.Error("missing expected attributes")
		}
	})

	t.Run("object with optional fields", func(t *testing.T) {
		v := byName["new_relic"]
		if v.Type.Kind != TypeObject {
			t.Errorf("type = %v, want object", v.Type.Kind)
		}
		if len(v.Type.Attributes) != 3 {
			t.Fatalf("attributes len = %d, want 3", len(v.Type.Attributes))
		}
		for _, attr := range v.Type.Attributes {
			if !attr.Optional {
				t.Errorf("attribute %q should be optional", attr.Name)
			}
		}
	})

	t.Run("has default flag", func(t *testing.T) {
		for _, v := range vars {
			if !v.HasDefault {
				t.Errorf("variable %q should have HasDefault=true", v.Name)
			}
		}
	})
}

func TestFormatDefault(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{nil, ""},
		{"hello", "hello"},
		{true, "true"},
		{false, "false"},
		{int64(42), "42"},
		{float64(3.14), "3.14"},
	}
	for _, tt := range tests {
		got := FormatDefault(tt.input)
		if got != tt.want {
			t.Errorf("FormatDefault(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
