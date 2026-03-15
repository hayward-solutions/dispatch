package engine

import "fmt"

// TypeKind describes the shape of a variable's value.
type TypeKind int

const (
	TypeString TypeKind = iota
	TypeNumber
	TypeBool
	TypeList
	TypeMap
	TypeObject
)

// String returns a human-readable name for the type kind.
func (k TypeKind) String() string {
	switch k {
	case TypeString:
		return "string"
	case TypeNumber:
		return "number"
	case TypeBool:
		return "bool"
	case TypeList:
		return "list"
	case TypeMap:
		return "map"
	case TypeObject:
		return "object"
	default:
		return "unknown"
	}
}

// VarType describes the full type of a variable, including nested types.
type VarType struct {
	Kind        TypeKind          // The kind of type
	ElementType *VarType          // For List and Map: the type of elements/values
	Attributes  []ObjectAttribute // For Object: the fields
}

// ObjectAttribute describes a single field within an Object type.
type ObjectAttribute struct {
	Name     string
	Type     VarType
	Optional bool
	Default  any
}

// Variable represents a parsed variable definition from an IaC engine.
type Variable struct {
	Name        string
	Description string
	Type        VarType
	Default     any  // nil if no default (i.e. required)
	HasDefault  bool // true when a default was explicitly set (even if the value is nil/empty)
}

// Engine parses variable definitions from a source file.
type Engine interface {
	ParseVariables(source []byte) ([]Variable, error)
}

// GetEngine returns an engine for the given mode name.
func GetEngine(mode string) (Engine, error) {
	switch mode {
	case "terraform":
		return &TerraformEngine{}, nil
	default:
		return nil, fmt.Errorf("unsupported engine mode: %s", mode)
	}
}
