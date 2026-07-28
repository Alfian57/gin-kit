package openapi

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

// timeType define package-level implementation state.
var timeType = reflect.TypeOf(time.Time{})

// schemaFor converts a Go value's type into a Schema, registering named
// struct types as components in defs and returning $ref schemas for them.
func schemaFor(value any, defs map[string]*Schema) *Schema {
	if value == nil {
		return &Schema{}
	}
	if t, ok := value.(reflect.Type); ok {
		return schemaForType(t, defs, map[reflect.Type]string{})
	}
	return schemaForType(reflect.TypeOf(value), defs, map[reflect.Type]string{})
}

// schemaForType performs this package operation.
func schemaForType(t reflect.Type, defs map[string]*Schema, seen map[reflect.Type]string) *Schema {
	switch t.Kind() {
	case reflect.Pointer:
		inner := schemaForType(t.Elem(), defs, seen)
		if inner.Ref != "" {
			// OpenAPI 3.0 cannot put nullable next to $ref directly.
			return &Schema{Nullable: true, AllOf: []*Schema{{Ref: inner.Ref}}}
		}
		inner.Nullable = true
		return inner
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return &Schema{Type: "integer"}
	case reflect.Int64, reflect.Uint64:
		return &Schema{Type: "integer", Format: "int64"}
	case reflect.Float32:
		return &Schema{Type: "number", Format: "float"}
	case reflect.Float64:
		return &Schema{Type: "number", Format: "double"}
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return &Schema{Type: "string", Format: "byte"}
		}
		return &Schema{Type: "array", Items: schemaForType(t.Elem(), defs, seen)}
	case reflect.Map:
		if t.Key().Kind() == reflect.String {
			return &Schema{Type: "object", AdditionalProperties: schemaForType(t.Elem(), defs, seen)}
		}
		return &Schema{Type: "object"}
	case reflect.Interface:
		return &Schema{}
	case reflect.Struct:
		if t == timeType {
			return &Schema{Type: "string", Format: "date-time"}
		}
		if t.Name() == "" {
			return structSchema(t, defs, seen)
		}
		if name, done := seen[t]; done {
			return &Schema{Ref: "#/components/schemas/" + name}
		}
		name := componentName(t, defs)
		// Register before recursing so self-referencing types terminate.
		seen[t] = name
		defs[name] = nil
		defs[name] = structSchema(t, defs, seen)
		return &Schema{Ref: "#/components/schemas/" + name}
	default:
		return &Schema{}
	}
}

// componentName picks a unique, exported-style component name for a struct
// type.
func componentName(t reflect.Type, defs map[string]*Schema) string {
	name := t.Name()
	name = strings.ToUpper(name[:1]) + name[1:]
	if _, taken := defs[name]; !taken {
		return name
	}
	parts := strings.Split(t.PkgPath(), "/")
	qualified := exportedName(parts[len(parts)-1]) + name
	if _, taken := defs[qualified]; !taken {
		return qualified
	}
	for i := 2; ; i++ {
		candidate := name + strconv.Itoa(i)
		if _, taken := defs[candidate]; !taken {
			return candidate
		}
	}
}

// exportedName performs this package operation.
func exportedName(pkg string) string {
	cleaned := strings.NewReplacer("-", "", "_", "", ".", "").Replace(pkg)
	if cleaned == "" {
		return ""
	}
	return strings.ToUpper(cleaned[:1]) + cleaned[1:]
}

// structSchema performs this package operation.
func structSchema(t reflect.Type, defs map[string]*Schema, seen map[reflect.Type]string) *Schema {
	schema := &Schema{Type: "object", Properties: map[string]*Schema{}}
	addStructFields(t, schema, defs, seen)
	return schema
}

// addStructFields performs this package operation.
func addStructFields(t reflect.Type, schema *Schema, defs map[string]*Schema, seen map[reflect.Type]string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if field.Anonymous && field.Type.Kind() == reflect.Struct && field.Type != timeType {
			addStructFields(field.Type, schema, defs, seen)
			continue
		}
		jsonTag := field.Tag.Get("json")
		name, options, _ := strings.Cut(jsonTag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		fieldSchema := schemaForType(field.Type, defs, seen)
		if strings.Contains(","+options+",", ",string,") {
			fieldSchema = &Schema{Type: "string"}
		}
		applyValidateTag(field.Tag.Get("validate"), fieldSchema, schema, name)
		schema.Properties[name] = fieldSchema
	}
}

// applyValidateTag lifts required/min/max constraints from validate tags into
// the schema, so generated inputs self-document.
func applyValidateTag(tag string, fieldSchema, parent *Schema, name string) {
	if tag == "" {
		return
	}
	for _, token := range strings.Split(tag, ",") {
		key, value, _ := strings.Cut(token, "=")
		switch key {
		case "required":
			parent.Required = append(parent.Required, name)
		case "min":
			if fieldSchema.Type == "string" {
				if parsed, err := strconv.Atoi(value); err == nil {
					fieldSchema.MinLength = &parsed
				}
			}
		case "max":
			if fieldSchema.Type == "string" {
				if parsed, err := strconv.Atoi(value); err == nil {
					fieldSchema.MaxLength = &parsed
				}
			}
		case "oneof":
			if fieldSchema.Type == "string" {
				fieldSchema.Enum = strings.Fields(value)
			}
		}
	}
}
