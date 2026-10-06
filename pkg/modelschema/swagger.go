package modelschema

import (
	"fmt"
	"go/ast"
)

// AddSwaggerDefinitions generates Swagger 2.0 (OpenAPI v2) definitions for the named type and every named struct type
// reachable from it, adds them to defs with the given name prefix (e.g. "imdl-v0.1.6.onpremisemodel."),
// and returns a schema referencing the named type.
func (p *Package) AddSwaggerDefinitions(defs map[string]interface{}, typeName, prefix string) (map[string]interface{}, error) {
	if !p.HasType(typeName) {
		return nil, fmt.Errorf("type %q is not defined", typeName)
	}
	g := &swaggerGenerator{
		pkg:    p,
		n:      &normalizer{pkg: p, fieldCache: map[*ast.StructType][]field{}},
		defs:   defs,
		prefix: prefix,
	}
	return g.schema(ast.NewIdent(typeName), 0), nil
}

type swaggerGenerator struct {
	pkg    *Package
	n      *normalizer
	defs   map[string]interface{}
	prefix string
}

func (g *swaggerGenerator) schema(t ast.Expr, depth int) map[string]interface{} {
	if depth > maxDepth {
		return map[string]interface{}{}
	}

	switch t := t.(type) {
	case *ast.ParenExpr:
		return g.schema(t.X, depth+1)
	case *ast.Ident:
		ts, ok := g.pkg.types[t.Name]
		if !ok {
			return basicSchema(t.Name)
		}
		if ts.TypeParams != nil {
			return map[string]interface{}{}
		}
		if g.n.localStruct(t) != nil {
			return g.define(t.Name, depth)
		}
		s := g.schema(ts.Type, depth+1)
		if values, ok := g.pkg.enums[t.Name]; ok && s["$ref"] == nil {
			s["enum"] = values
		}
		return s
	case *ast.SelectorExpr:
		pkgName := ""
		if id, ok := t.X.(*ast.Ident); ok {
			pkgName = id.Name
		}
		switch pkgName + "." + t.Sel.Name {
		case "time.Time":
			return map[string]interface{}{"type": "string", "format": "date-time"}
		case "time.Duration":
			return map[string]interface{}{"type": "integer"}
		}
		return map[string]interface{}{}
	case *ast.StarExpr:
		return g.schema(t.X, depth+1)
	case *ast.ArrayType:
		if t.Len == nil && isByteElem(t.Elt) {
			return map[string]interface{}{"type": "string", "format": "byte"}
		}
		return map[string]interface{}{"type": "array", "items": g.schema(t.Elt, depth+1)}
	case *ast.MapType:
		return map[string]interface{}{"type": "object", "additionalProperties": g.schema(t.Value, depth+1)}
	case *ast.StructType:
		return g.structSchema(t, depth)
	default:
		return map[string]interface{}{}
	}
}

func basicSchema(name string) map[string]interface{} {
	switch name {
	case "string":
		return map[string]interface{}{"type": "string"}
	case "bool":
		return map[string]interface{}{"type": "boolean"}
	case "int", "int8", "int16", "int32", "int64", "rune",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "byte":
		return map[string]interface{}{"type": "integer"}
	case "float32", "float64":
		return map[string]interface{}{"type": "number"}
	}
	return map[string]interface{}{}
}

// define adds the definition of a named struct type (once) and returns a reference to it.
func (g *swaggerGenerator) define(name string, depth int) map[string]interface{} {
	fullName := g.prefix + name
	ref := map[string]interface{}{"$ref": "#/definitions/" + fullName}
	if _, exists := g.defs[fullName]; exists {
		return ref
	}

	// Register a placeholder first so that recursive types refer to the same definition.
	def := map[string]interface{}{}
	g.defs[fullName] = def
	for k, v := range g.structSchema(g.n.localStruct(ast.NewIdent(name)), depth+1) {
		def[k] = v
	}
	return ref
}

func (g *swaggerGenerator) structSchema(st *ast.StructType, depth int) map[string]interface{} {
	properties := map[string]interface{}{}
	var required []string
	for _, f := range g.n.fields(st) {
		s := g.schema(f.typ, depth+1)
		if f.doc != "" {
			if _, isRef := s["$ref"]; isRef {
				s = map[string]interface{}{"description": f.doc, "allOf": []interface{}{s}}
			} else {
				s["description"] = f.doc
			}
		}
		properties[f.name] = s
		if f.required {
			required = append(required, f.name)
		}
	}

	out := map[string]interface{}{"type": "object", "properties": properties}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}
