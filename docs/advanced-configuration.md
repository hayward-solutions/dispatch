# Advanced Configuration

Advanced mode provides a typed, step-based configuration UI for repositories with Infrastructure as Code (IaC) — such as Terraform. Instead of managing flat key-value environment variables, Dispatch reads your variable definitions and presents a structured form with proper type support.

## When to Use Advanced Mode

| Use **Simple** mode when... | Use **Advanced** mode when... |
|-----------------------------|-------------------------------|
| Variables are plain strings (API keys, feature flags) | Variables have complex types (lists, maps, objects) |
| No IaC tooling involved | Terraform or similar IaC manages the infrastructure |
| Teams manage a small number of variables | Configuration is grouped into logical steps |
| No variable type validation needed | Type safety and structured input matters |

## How It Works

Dispatch automatically detects advanced mode by looking for a `.dispatch.yaml` file in the repository root. When present:

1. The repository is marked as **Advanced** in the UI
2. Environment creation skips inline variable/secret editing — configuration happens after creation through the step-based flow
3. The environment detail page shows an accordion-style configuration panel with steps defined in `.dispatch.yaml`
4. Variable types are parsed from the IaC source file (e.g., `variables.tf`), and the UI renders appropriate form fields for each type

## Setting Up `.dispatch.yaml`

Create a `.dispatch.yaml` file in your repository root:

```yaml
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
    description: Allow GitHub repositories access to this account with Actions OIDC.
    inputs:
      - terraform_repositories
      - ecr_repositories

  - step: Monitoring
    inputs:
      - new_relic
```

### Schema Reference

| Field | Required | Description |
|-------|----------|-------------|
| `mode` | Yes | Engine type. Currently only `"terraform"` is supported. |
| `path_to_tfvars` | Yes | Path to the variable definitions file, relative to the repo root (e.g., `terraform/variables.tf`). |
| `ignore_inputs` | No | List of variable names to exclude from the UI. Use this for variables that are set by the workflow itself (e.g., `region`, `assume_role_arns`). |
| `flow` | Yes | Array of steps. At least one step is required. |

Each step in `flow`:

| Field | Required | Description |
|-------|----------|-------------|
| `step` | Yes | Display name for the step (shown in the accordion header). |
| `description` | No | Optional description shown below the step title. |
| `inputs` | Yes | List of variable names to include in this step. Names must match variables defined in the file referenced by `path_to_tfvars`. |

### Validation Rules

- `mode` must be `"terraform"`
- `path_to_tfvars` must be a non-empty path
- `flow` must contain at least one step
- Each step must have a non-empty `step` name and at least one entry in `inputs`

## Supported Variable Types

Dispatch parses Terraform variable definitions and renders type-appropriate form fields:

| Terraform Type | UI Control | Storage Format |
|----------------|------------|----------------|
| `string` | Text input | Plain string |
| `number` | Number input | Numeric string |
| `bool` | Toggle checkbox | `"true"` / `"false"` |
| `list(string)` | Dynamic list with add/remove | JSON array: `["a", "b"]` |
| `map(...)` | Key-value cards with add/remove | JSON object: `{"key": {...}}` |
| `object({...})` | Structured form with typed fields | JSON object: `{"field": "value"}` |

Nested types are supported — for example, `map(object({ github_org = optional(string), github_repo = string }))` renders as expandable cards with typed fields for each attribute.

Variables with `default` values are shown but not required. Variables without defaults are marked as required in the UI.

## How Variables Are Stored

When a user saves a step, each variable is stored as a GitHub environment variable with a `TF_VAR_` prefix:

| Terraform Variable | GitHub Environment Variable |
|---|---|
| `additional_regions` | `TF_VAR_ADDITIONAL_REGIONS` |
| `terraform_repositories` | `TF_VAR_TERRAFORM_REPOSITORIES` |
| `new_relic` | `TF_VAR_NEW_RELIC` |

> **Note:** GitHub's API uppercases all environment variable names. Dispatch handles this automatically when reading values back, but your workflow needs to account for it when exporting variables for Terraform (see below).

## Workflow Integration

Terraform expects `TF_VAR_` environment variables with **lowercase** variable names (e.g., `TF_VAR_additional_regions`), but GitHub stores them uppercased (e.g., `TF_VAR_ADDITIONAL_REGIONS`).

Add this step to your workflow to bridge the gap:

```yaml
- name: Export variables
  run: |
    echo '${{ toJSON(vars) }}' | jq -r 'to_entries[] | select(.key | startswith("TF_VAR_")) | "TF_VAR_\(.key[7:] | ascii_downcase)=\(.value)"' >> "$GITHUB_ENV"
```

This step:

1. Reads all GitHub environment variables as JSON
2. Filters to only `TF_VAR_` prefixed variables
3. Keeps the `TF_VAR_` prefix uppercase
4. Lowercases the variable name portion
5. Exports them to `$GITHUB_ENV` so Terraform picks them up automatically

With this in place, your Terraform commands need no `-var` flags:

```yaml
- name: Plan
  run: terraform plan -no-color

- name: Apply
  run: terraform apply -auto-approve
```

### Full Workflow Example

```yaml
name: Pull Request
on:
  pull_request:
    branches: [main]

jobs:
  plan:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4

      - uses: hashicorp/setup-terraform@v3

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_ROLE_ARN }}
          aws-region: us-east-1

      - name: Init
        run: terraform init

      - name: Export variables
        run: |
          echo '${{ toJSON(vars) }}' | jq -r 'to_entries[] | select(.key | startswith("TF_VAR_")) | "TF_VAR_\(.key[7:] | ascii_downcase)=\(.value)"' >> "$GITHUB_ENV"

      - name: Plan
        run: terraform plan -no-color
```
