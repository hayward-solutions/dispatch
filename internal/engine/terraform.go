package engine

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// TerraformEngine parses Terraform variable definitions from .tf files.
type TerraformEngine struct{}

func (e *TerraformEngine) ParseVariables(source []byte) ([]Variable, error) {
	file, diags := hclsyntax.ParseConfig(source, "variables.tf", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse HCL: %s", diags.Error())
	}

	body := file.Body.(*hclsyntax.Body)

	var variables []Variable
	for _, block := range body.Blocks {
		if block.Type != "variable" || len(block.Labels) == 0 {
			continue
		}

		v := Variable{
			Name: block.Labels[0],
			Type: VarType{Kind: TypeString}, // default type
		}

		attrs, _ := block.Body.JustAttributes()

		// Description
		if attr, ok := attrs["description"]; ok {
			val, diags := attr.Expr.Value(nil)
			if !diags.HasErrors() && val.Type() == cty.String {
				v.Description = val.AsString()
			}
		}

		// Type — use TypeConstraintWithDefaults to support optional(type, default)
		if attr, ok := attrs["type"]; ok {
			ty, defaults, diags := typeexpr.TypeConstraintWithDefaults(attr.Expr)
			if !diags.HasErrors() {
				v.Type = ctyTypeToVarType(ty)
				applyOptionalDefaults(&v.Type, defaults)
			}
		}

		// Default
		if attr, ok := attrs["default"]; ok {
			val, diags := attr.Expr.Value(nil)
			if !diags.HasErrors() {
				v.Default = ctyValueToGo(val)
				v.HasDefault = true
			}
		}

		variables = append(variables, v)
	}

	sort.Slice(variables, func(i, j int) bool {
		return variables[i].Name < variables[j].Name
	})

	return variables, nil
}

// applyOptionalDefaults walks the VarType tree and sets Default values on
// ObjectAttributes from the typeexpr.Defaults information.
func applyOptionalDefaults(vt *VarType, defaults *typeexpr.Defaults) {
	if defaults == nil {
		return
	}
	if vt.Kind == TypeObject {
		for i := range vt.Attributes {
			if dv, ok := defaults.DefaultValues[vt.Attributes[i].Name]; ok {
				vt.Attributes[i].Default = ctyValueToGo(dv)
			}
			// Recurse into children
			if child, ok := defaults.Children[vt.Attributes[i].Name]; ok {
				applyOptionalDefaults(&vt.Attributes[i].Type, child)
			}
		}
	}
	if vt.ElementType != nil {
		// For map/list of objects, the child key is empty string
		if child, ok := defaults.Children[""]; ok {
			applyOptionalDefaults(vt.ElementType, child)
		}
	}
}

// ctyTypeToVarType converts a cty.Type to our VarType representation.
func ctyTypeToVarType(ty cty.Type) VarType {
	switch {
	case ty == cty.String:
		return VarType{Kind: TypeString}
	case ty == cty.Number:
		return VarType{Kind: TypeNumber}
	case ty == cty.Bool:
		return VarType{Kind: TypeBool}
	case ty.IsListType():
		elem := ctyTypeToVarType(ty.ElementType())
		return VarType{Kind: TypeList, ElementType: &elem}
	case ty.IsSetType():
		elem := ctyTypeToVarType(ty.ElementType())
		return VarType{Kind: TypeList, ElementType: &elem}
	case ty.IsMapType():
		elem := ctyTypeToVarType(ty.ElementType())
		return VarType{Kind: TypeMap, ElementType: &elem}
	case ty.IsObjectType():
		attrs := objectAttributes(ty)
		return VarType{Kind: TypeObject, Attributes: attrs}
	case ty.IsTupleType():
		return VarType{Kind: TypeList, ElementType: &VarType{Kind: TypeString}}
	default:
		// Dynamic or unknown types fall back to string
		return VarType{Kind: TypeString}
	}
}

func objectAttributes(ty cty.Type) []ObjectAttribute {
	attrTypes := ty.AttributeTypes()
	names := make([]string, 0, len(attrTypes))
	for name := range attrTypes {
		names = append(names, name)
	}
	sort.Strings(names)

	attrs := make([]ObjectAttribute, 0, len(names))
	for _, name := range names {
		attrType := attrTypes[name]
		attr := ObjectAttribute{
			Name:     name,
			Type:     ctyTypeToVarType(attrType),
			Optional: ty.AttributeOptional(name),
		}
		attrs = append(attrs, attr)
	}
	return attrs
}

// ctyValueToGo converts a cty.Value to a native Go value suitable for JSON serialization.
func ctyValueToGo(val cty.Value) any {
	if !val.IsKnown() || val.IsNull() {
		return nil
	}

	ty := val.Type()

	switch {
	case ty == cty.String:
		return val.AsString()
	case ty == cty.Bool:
		return val.True()
	case ty == cty.Number:
		bf := val.AsBigFloat()
		if bf.IsInt() {
			i, _ := bf.Int64()
			return i
		}
		f, _ := bf.Float64()
		return f
	case ty.IsListType() || ty.IsSetType() || ty.IsTupleType():
		var items []any
		for it := val.ElementIterator(); it.Next(); {
			_, v := it.Element()
			items = append(items, ctyValueToGo(v))
		}
		if items == nil {
			items = []any{}
		}
		return items
	case ty.IsMapType() || ty.IsObjectType():
		m := make(map[string]any)
		for it := val.ElementIterator(); it.Next(); {
			k, v := it.Element()
			m[k.AsString()] = ctyValueToGo(v)
		}
		return m
	default:
		return nil
	}
}

// GoValueToJSON converts a Go value to a JSON string for storage in GitHub env vars.
func GoValueToJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// JSONToGoValue converts a JSON string back to a Go value.
func JSONToGoValue(s string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	// json.Unmarshal uses float64 for numbers; convert whole numbers to int64
	return normalizeJSONNumbers(v), nil
}

func normalizeJSONNumbers(v any) any {
	switch val := v.(type) {
	case float64:
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	case map[string]any:
		for k, v := range val {
			val[k] = normalizeJSONNumbers(v)
		}
		return val
	case []any:
		for i, v := range val {
			val[i] = normalizeJSONNumbers(v)
		}
		return val
	default:
		return v
	}
}

// FormatDefault returns a human-readable string representation of a default value.
func FormatDefault(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case *big.Float:
		return val.Text('g', -1)
	default:
		b, _ := json.MarshalIndent(val, "", "  ")
		return string(b)
	}
}
