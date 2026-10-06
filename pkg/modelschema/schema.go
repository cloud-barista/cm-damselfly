package modelschema

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxIssues = 50
	maxDepth  = 128
)

// Package holds the top-level type declarations of a parsed Go package.
type Package struct {
	types map[string]*ast.TypeSpec
	// enums holds the values of typed constants (e.g. 'const StatusOK Status = "ok"') per type name.
	enums map[string][]interface{}
}

// ParsePackage parses Go source files (file name -> content) of a single package.
func ParsePackage(files map[string][]byte) (*Package, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	fset := token.NewFileSet()
	pkg := &Package{types: map[string]*ast.TypeSpec{}, enums: map[string][]interface{}{}}
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, files[name], parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			if gd.Tok == token.CONST {
				pkg.collectEnums(gd)
				continue
			}
			if gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					pkg.types[ts.Name.Name] = ts
				}
			}
		}
	}
	return pkg, nil
}

// collectEnums records typed constants with a literal value, which are used as enum values of their type.
func (p *Package) collectEnums(gd *ast.GenDecl) {
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		typeIdent, ok := vs.Type.(*ast.Ident)
		if !ok || len(vs.Names) != len(vs.Values) {
			continue
		}
		for _, value := range vs.Values {
			lit, ok := value.(*ast.BasicLit)
			if !ok {
				continue
			}
			switch lit.Kind {
			case token.STRING:
				if str, err := strconv.Unquote(lit.Value); err == nil {
					p.enums[typeIdent.Name] = append(p.enums[typeIdent.Name], str)
				}
			case token.INT:
				if i, err := strconv.ParseInt(lit.Value, 0, 64); err == nil {
					p.enums[typeIdent.Name] = append(p.enums[typeIdent.Name], i)
				}
			}
		}
	}
}

// HasType reports whether the package declares the named type.
func (p *Package) HasType(name string) bool {
	_, ok := p.types[name]
	return ok
}

// ValidationError lists the mismatches between a JSON document and the Go type definition.
type ValidationError struct {
	Issues []string
	Total  int
}

func (e *ValidationError) Error() string {
	msg := strings.Join(e.Issues, "; ")
	if e.Total > len(e.Issues) {
		msg += fmt.Sprintf("; ... and %d more", e.Total-len(e.Issues))
	}
	return msg
}

// Normalize validates the JSON document against the named type and returns it as encoding/json would
// produce after unmarshalling into and marshalling from that type: unknown fields and type mismatches are
// reported as a *ValidationError, keys are canonicalized and missing fields are filled with zero values.
// rootPath is used as a prefix in validation messages (e.g. "onpremiseInfraModel").
func (p *Package) Normalize(data []byte, typeName, rootPath string) (json.RawMessage, error) {
	if !p.HasType(typeName) {
		return nil, fmt.Errorf("type %q is not defined", typeName)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, &ValidationError{Issues: []string{fmt.Sprintf("%s: invalid JSON: %v", rootPath, err)}, Total: 1}
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, &ValidationError{Issues: []string{fmt.Sprintf("%s: invalid JSON: unexpected data after the top-level value", rootPath)}, Total: 1}
	}

	n := &normalizer{pkg: p, fieldCache: map[*ast.StructType][]field{}}
	out := n.value(v, ast.NewIdent(typeName), rootPath, 0)
	if n.total > 0 {
		return nil, &ValidationError{Issues: n.issues, Total: n.total}
	}
	return json.Marshal(out)
}

// structObject is a normalized Go struct value. It is distinguished from a map value because
// structs are never considered empty for 'omitempty'.
type structObject map[string]interface{}

type field struct {
	name      string
	tagged    bool
	omitEmpty bool
	omitZero  bool
	required  bool
	doc       string
	typ       ast.Expr
	depth     int
	// ptrChains identifies the embedded pointer structs this (promoted) field belongs to.
	ptrChains []string
}

type normalizer struct {
	pkg        *Package
	fieldCache map[*ast.StructType][]field
	issues     []string
	total      int
}

func (n *normalizer) addf(path, format string, args ...interface{}) {
	n.total++
	if len(n.issues) < maxIssues {
		n.issues = append(n.issues, path+": "+fmt.Sprintf(format, args...))
	}
}

func (n *normalizer) mismatch(path, expected string, v interface{}) {
	n.addf(path, "expected %s, got %s", expected, jsonKind(v))
}

func jsonKind(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case json.Number:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	}
	return fmt.Sprintf("%T", v)
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// value normalizes v as the Go type t. A nil v (JSON null or missing) yields the zero value of t.
func (n *normalizer) value(v interface{}, t ast.Expr, path string, depth int) interface{} {
	if depth > maxDepth {
		n.addf(path, "exceeds the maximum nesting depth")
		return nil
	}

	switch t := t.(type) {
	case *ast.ParenExpr:
		return n.value(v, t.X, path, depth+1)
	case *ast.Ident:
		if ts, ok := n.pkg.types[t.Name]; ok {
			if ts.TypeParams != nil {
				return v
			}
			return n.value(v, ts.Type, path, depth+1)
		}
		return n.basic(v, t.Name, path)
	case *ast.SelectorExpr:
		return n.external(v, t, path)
	case *ast.StarExpr:
		if v == nil {
			return nil
		}
		return n.value(v, t.X, path, depth+1)
	case *ast.ArrayType:
		return n.array(v, t, path, depth)
	case *ast.MapType:
		return n.mapValue(v, t, path, depth)
	case *ast.StructType:
		return n.structValue(v, t, path, depth)
	default:
		// interface{}, generic instantiations and other types are not constrained
		return v
	}
}

func (n *normalizer) basic(v interface{}, name, path string) interface{} {
	bits := 64
	switch name {
	case "int8", "uint8", "byte":
		bits = 8
	case "int16", "uint16":
		bits = 16
	case "int32", "uint32", "rune", "float32":
		bits = 32
	}

	switch name {
	case "string":
		if v == nil {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			n.mismatch(path, "string", v)
			return ""
		}
		return s
	case "bool":
		if v == nil {
			return false
		}
		b, ok := v.(bool)
		if !ok {
			n.mismatch(path, "boolean", v)
			return false
		}
		return b
	case "int", "int8", "int16", "int32", "int64", "rune":
		if v == nil {
			return json.Number("0")
		}
		num, ok := v.(json.Number)
		if !ok {
			n.mismatch(path, "integer", v)
			return json.Number("0")
		}
		i, err := strconv.ParseInt(num.String(), 10, bits)
		if err != nil {
			n.addf(path, "%s is not a valid %s value", num, name)
			return json.Number("0")
		}
		return json.Number(strconv.FormatInt(i, 10))
	case "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "byte":
		if v == nil {
			return json.Number("0")
		}
		num, ok := v.(json.Number)
		if !ok {
			n.mismatch(path, "non-negative integer", v)
			return json.Number("0")
		}
		u, err := strconv.ParseUint(num.String(), 10, bits)
		if err != nil {
			n.addf(path, "%s is not a valid %s value", num, name)
			return json.Number("0")
		}
		return json.Number(strconv.FormatUint(u, 10))
	case "float32", "float64":
		if v == nil {
			return json.Number("0")
		}
		num, ok := v.(json.Number)
		if !ok {
			n.mismatch(path, "number", v)
			return json.Number("0")
		}
		if _, err := strconv.ParseFloat(num.String(), bits); err != nil {
			n.addf(path, "%s is not a valid %s value", num, name)
			return json.Number("0")
		}
		return num
	case "complex64", "complex128":
		n.addf(path, "type %s is not supported by JSON", name)
		return nil
	default:
		// any, error and identifiers that cannot be resolved are not constrained
		return v
	}
}

func (n *normalizer) external(v interface{}, t *ast.SelectorExpr, path string) interface{} {
	pkgName := ""
	if id, ok := t.X.(*ast.Ident); ok {
		pkgName = id.Name
	}

	switch pkgName + "." + t.Sel.Name {
	case "time.Time":
		if v == nil {
			return time.Time{}.Format(time.RFC3339Nano)
		}
		s, ok := v.(string)
		if !ok {
			n.mismatch(path, "RFC 3339 time string", v)
			return time.Time{}.Format(time.RFC3339Nano)
		}
		tm, err := time.Parse(time.RFC3339, s)
		if err != nil {
			n.addf(path, "%q is not a valid RFC 3339 time", s)
			return time.Time{}.Format(time.RFC3339Nano)
		}
		return tm.Format(time.RFC3339Nano)
	case "time.Duration":
		return n.basic(v, "int64", path)
	default:
		return v
	}
}

func isByteElem(t ast.Expr) bool {
	id, ok := t.(*ast.Ident)
	return ok && (id.Name == "byte" || id.Name == "uint8")
}

func (n *normalizer) array(v interface{}, t *ast.ArrayType, path string, depth int) interface{} {
	size := -1
	if t.Len != nil {
		if lit, ok := t.Len.(*ast.BasicLit); ok && lit.Kind == token.INT {
			if l, err := strconv.Atoi(lit.Value); err == nil {
				size = l
			}
		}
	}

	if size < 0 {
		if v == nil {
			return nil
		}
		if isByteElem(t.Elt) {
			s, ok := v.(string)
			if !ok {
				n.mismatch(path, "base64 string", v)
				return nil
			}
			if _, err := base64.StdEncoding.DecodeString(s); err != nil {
				n.addf(path, "invalid base64 string")
			}
			return s
		}
		arr, ok := v.([]interface{})
		if !ok {
			n.mismatch(path, "array", v)
			return nil
		}
		out := make([]interface{}, len(arr))
		for i, elem := range arr {
			out[i] = n.value(elem, t.Elt, fmt.Sprintf("%s[%d]", path, i), depth+1)
		}
		return out
	}

	var arr []interface{}
	if v != nil {
		var ok bool
		if arr, ok = v.([]interface{}); !ok {
			n.mismatch(path, "array", v)
		}
	}
	out := make([]interface{}, size)
	for i := 0; i < size; i++ {
		var elem interface{}
		if i < len(arr) {
			elem = arr[i]
		}
		out[i] = n.value(elem, t.Elt, fmt.Sprintf("%s[%d]", path, i), depth+1)
	}
	return out
}

// underlyingBasic resolves local named types to the name of their underlying predeclared type.
func (n *normalizer) underlyingBasic(t ast.Expr) string {
	for i := 0; i < maxDepth; i++ {
		switch tt := t.(type) {
		case *ast.ParenExpr:
			t = tt.X
		case *ast.Ident:
			ts, ok := n.pkg.types[tt.Name]
			if !ok {
				return tt.Name
			}
			t = ts.Type
		default:
			return ""
		}
	}
	return ""
}

func (n *normalizer) mapValue(v interface{}, t *ast.MapType, path string, depth int) interface{} {
	if v == nil {
		return nil
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		n.mismatch(path, "object", v)
		return nil
	}

	keyKind := n.underlyingBasic(t.Key)
	keys := sortedKeys(obj)
	out := make(map[string]interface{}, len(obj))
	for _, k := range keys {
		outKey := k
		switch keyKind {
		case "string":
		case "int", "int8", "int16", "int32", "int64", "rune":
			i, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				n.addf(path, "map key %q is not a valid %s", k, keyKind)
				continue
			}
			outKey = strconv.FormatInt(i, 10)
		case "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "byte":
			u, err := strconv.ParseUint(k, 10, 64)
			if err != nil {
				n.addf(path, "map key %q is not a valid %s", k, keyKind)
				continue
			}
			outKey = strconv.FormatUint(u, 10)
		default:
			n.addf(path, "unsupported map key type")
			return nil
		}
		out[outKey] = n.value(obj[k], t.Value, joinPath(path, k), depth+1)
	}
	return out
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// localStruct resolves t (through local named types) to a struct type, or returns nil.
func (n *normalizer) localStruct(t ast.Expr) *ast.StructType {
	for i := 0; i < maxDepth; i++ {
		switch tt := t.(type) {
		case *ast.ParenExpr:
			t = tt.X
		case *ast.StructType:
			return tt
		case *ast.Ident:
			ts, ok := n.pkg.types[tt.Name]
			if !ok || ts.TypeParams != nil {
				return nil
			}
			t = ts.Type
		default:
			return nil
		}
	}
	return nil
}

func embeddedTypeName(t ast.Expr) string {
	switch tt := t.(type) {
	case *ast.Ident:
		return tt.Name
	case *ast.SelectorExpr:
		return tt.Sel.Name
	case *ast.IndexExpr:
		return embeddedTypeName(tt.X)
	case *ast.IndexListExpr:
		return embeddedTypeName(tt.X)
	}
	return ""
}

// fieldDoc returns the doc comment (or line comment) of a struct field.
func fieldDoc(f *ast.Field) string {
	if f.Doc != nil {
		return strings.TrimSpace(f.Doc.Text())
	}
	if f.Comment != nil {
		return strings.TrimSpace(f.Comment.Text())
	}
	return ""
}

// isRequiredField reports whether the field has a 'validate:"required"' or 'binding:"required"' tag.
func isRequiredField(f *ast.Field) bool {
	if f.Tag == nil {
		return false
	}
	raw, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return false
	}
	for _, key := range []string{"validate", "binding"} {
		for _, opt := range strings.Split(reflect.StructTag(raw).Get(key), ",") {
			if opt == "required" {
				return true
			}
		}
	}
	return false
}

func parseJSONTag(f *ast.Field) (name string, opts []string, skip bool) {
	if f.Tag == nil {
		return "", nil, false
	}
	raw, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return "", nil, false
	}
	tag := reflect.StructTag(raw).Get("json")
	if tag == "-" {
		return "", nil, true
	}
	parts := strings.Split(tag, ",")
	return parts[0], parts[1:], false
}

func hasOption(opts []string, opt string) bool {
	for _, o := range opts {
		if o == opt {
			return true
		}
	}
	return false
}

func (n *normalizer) collectFields(st *ast.StructType, depth int, ptrChains []string, chainID string, visiting map[*ast.StructType]bool, out *[]field) {
	if visiting[st] {
		return
	}
	visiting[st] = true
	defer delete(visiting, st)

	for fi, f := range st.Fields.List {
		tagName, opts, skip := parseJSONTag(f)
		if skip {
			continue
		}
		newField := func(name string) field {
			return field{
				name:      name,
				tagged:    tagName != "",
				omitEmpty: hasOption(opts, "omitempty"),
				omitZero:  hasOption(opts, "omitzero"),
				required:  isRequiredField(f),
				doc:       fieldDoc(f),
				typ:       f.Type,
				depth:     depth,
				ptrChains: ptrChains,
			}
		}

		if len(f.Names) == 0 {
			typ, isPtr := f.Type, false
			if star, ok := typ.(*ast.StarExpr); ok {
				typ, isPtr = star.X, true
			}
			typeName := embeddedTypeName(typ)

			if tagName == "" {
				if embedded := n.localStruct(typ); embedded != nil {
					id := fmt.Sprintf("%s/%d", chainID, fi)
					chains := ptrChains
					if isPtr {
						chains = append(append([]string(nil), ptrChains...), id)
					}
					n.collectFields(embedded, depth+1, chains, id, visiting, out)
					continue
				}
			}
			if !ast.IsExported(typeName) {
				continue
			}
			name := tagName
			if name == "" {
				name = typeName
			}
			*out = append(*out, newField(name))
			continue
		}

		for _, id := range f.Names {
			if !id.IsExported() {
				continue
			}
			name := tagName
			if name == "" {
				name = id.Name
			}
			*out = append(*out, newField(name))
		}
	}
}

// fields returns the JSON fields of the struct applying encoding/json's rules for promoted fields.
func (n *normalizer) fields(st *ast.StructType) []field {
	if cached, ok := n.fieldCache[st]; ok {
		return cached
	}

	var all []field
	n.collectFields(st, 0, nil, "", map[*ast.StructType]bool{}, &all)

	var names []string
	byName := map[string][]field{}
	for _, f := range all {
		if _, ok := byName[f.name]; !ok {
			names = append(names, f.name)
		}
		byName[f.name] = append(byName[f.name], f)
	}

	var result []field
	for _, name := range names {
		candidates := byName[name]
		minDepth := candidates[0].depth
		for _, c := range candidates {
			if c.depth < minDepth {
				minDepth = c.depth
			}
		}
		var shallow, tagged []field
		for _, c := range candidates {
			if c.depth == minDepth {
				shallow = append(shallow, c)
				if c.tagged {
					tagged = append(tagged, c)
				}
			}
		}
		if len(shallow) == 1 {
			result = append(result, shallow[0])
		} else if len(tagged) == 1 {
			result = append(result, tagged[0])
		}
	}

	n.fieldCache[st] = result
	return result
}

func matchField(fields []field, key string) int {
	for i, f := range fields {
		if f.name == key {
			return i
		}
	}
	for i, f := range fields {
		if strings.EqualFold(f.name, key) {
			return i
		}
	}
	return -1
}

func isEmptyValue(v interface{}) bool {
	switch vv := v.(type) {
	case nil:
		return true
	case bool:
		return !vv
	case string:
		return vv == ""
	case json.Number:
		f, err := vv.Float64()
		return err == nil && f == 0
	case []interface{}:
		return len(vv) == 0
	case map[string]interface{}:
		return len(vv) == 0
	}
	return false
}

func (n *normalizer) structValue(v interface{}, st *ast.StructType, path string, depth int) interface{} {
	fields := n.fields(st)

	var obj map[string]interface{}
	if v != nil {
		var ok bool
		if obj, ok = v.(map[string]interface{}); !ok {
			n.mismatch(path, "object", v)
		}
	}

	values := map[int]interface{}{}
	allocated := map[string]bool{}
	for _, k := range sortedKeys(obj) {
		idx := matchField(fields, k)
		if idx < 0 {
			n.addf(joinPath(path, k), "unknown field")
			continue
		}
		values[idx] = n.value(obj[k], fields[idx].typ, joinPath(path, fields[idx].name), depth+1)
		for _, c := range fields[idx].ptrChains {
			allocated[c] = true
		}
	}

	out := structObject{}
	for i, f := range fields {
		skip := false
		for _, c := range f.ptrChains {
			if !allocated[c] {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		val, ok := values[i]
		if !ok {
			val = n.value(nil, f.typ, joinPath(path, f.name), depth+1)
		}
		if f.omitEmpty && isEmptyValue(val) {
			continue
		}
		if f.omitZero && reflect.DeepEqual(val, n.value(nil, f.typ, joinPath(path, f.name), depth+1)) {
			continue
		}
		out[f.name] = val
	}
	return out
}
