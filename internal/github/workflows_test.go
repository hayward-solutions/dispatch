package github

import (
	"testing"
)

func TestExtractDispatchInputs_ScalarOn(t *testing.T) {
	yaml := []byte(`
name: My Workflow
on: workflow_dispatch
jobs:
  build:
    runs-on: ubuntu-latest
`)
	inputs, ok := extractDispatchInputs(yaml)
	if !ok {
		t.Fatal("expected workflow_dispatch to be detected")
	}
	if len(inputs) != 0 {
		t.Errorf("expected 0 inputs, got %d", len(inputs))
	}
}

func TestExtractDispatchInputs_SequenceOn(t *testing.T) {
	yaml := []byte(`
name: My Workflow
on: [push, workflow_dispatch]
jobs:
  build:
    runs-on: ubuntu-latest
`)
	inputs, ok := extractDispatchInputs(yaml)
	if !ok {
		t.Fatal("expected workflow_dispatch to be detected")
	}
	if len(inputs) != 0 {
		t.Errorf("expected 0 inputs, got %d", len(inputs))
	}
}

func TestExtractDispatchInputs_SequenceWithoutDispatch(t *testing.T) {
	yaml := []byte(`
name: My Workflow
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
`)
	_, ok := extractDispatchInputs(yaml)
	if ok {
		t.Fatal("expected workflow_dispatch to NOT be detected")
	}
}

func TestExtractDispatchInputs_MappingWithInputs(t *testing.T) {
	yaml := []byte(`
name: Deploy
on:
  workflow_dispatch:
    inputs:
      environment:
        description: Target environment
        required: true
        type: choice
        options:
          - staging
          - production
      dry_run:
        description: Dry run mode
        type: boolean
        default: "false"
      version:
        description: Version to deploy
        required: true
`)
	inputs, ok := extractDispatchInputs(yaml)
	if !ok {
		t.Fatal("expected workflow_dispatch to be detected")
	}
	if len(inputs) != 3 {
		t.Fatalf("expected 3 inputs, got %d", len(inputs))
	}

	// Inputs should be sorted by name
	if inputs[0].Name != "dry_run" {
		t.Errorf("expected first input 'dry_run', got %q", inputs[0].Name)
	}
	if inputs[1].Name != "environment" {
		t.Errorf("expected second input 'environment', got %q", inputs[1].Name)
	}
	if inputs[2].Name != "version" {
		t.Errorf("expected third input 'version', got %q", inputs[2].Name)
	}

	// Check environment input
	env := inputs[1]
	if env.Description != "Target environment" {
		t.Errorf("unexpected description: %s", env.Description)
	}
	if !env.Required {
		t.Error("expected required=true")
	}
	if env.Type != "choice" {
		t.Errorf("expected type 'choice', got %q", env.Type)
	}
	if len(env.Options) != 2 || env.Options[0] != "staging" || env.Options[1] != "production" {
		t.Errorf("unexpected options: %v", env.Options)
	}

	// Check version input defaults to "string" type
	ver := inputs[2]
	if ver.Type != "string" {
		t.Errorf("expected default type 'string', got %q", ver.Type)
	}
}

func TestExtractDispatchInputs_MappingWithoutInputs(t *testing.T) {
	yaml := []byte(`
name: My Workflow
on:
  workflow_dispatch:
jobs:
  build:
    runs-on: ubuntu-latest
`)
	inputs, ok := extractDispatchInputs(yaml)
	if !ok {
		t.Fatal("expected workflow_dispatch to be detected")
	}
	if len(inputs) != 0 {
		t.Errorf("expected 0 inputs, got %d", len(inputs))
	}
}

func TestExtractDispatchInputs_NoDispatch(t *testing.T) {
	yaml := []byte(`
name: CI
on:
  push:
    branches: [main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
`)
	_, ok := extractDispatchInputs(yaml)
	if ok {
		t.Fatal("expected workflow_dispatch NOT to be detected")
	}
}

func TestExtractDispatchInputs_InvalidYAML(t *testing.T) {
	yaml := []byte(`
this is not: [valid yaml
  broken: {{{}}}
`)
	_, ok := extractDispatchInputs(yaml)
	if ok {
		t.Fatal("expected false for invalid YAML")
	}
}

func TestExtractDispatchInputs_ScalarNonDispatch(t *testing.T) {
	yaml := []byte(`
name: My Workflow
on: push
jobs:
  build:
    runs-on: ubuntu-latest
`)
	_, ok := extractDispatchInputs(yaml)
	if ok {
		t.Fatal("expected workflow_dispatch NOT to be detected for on: push")
	}
}
