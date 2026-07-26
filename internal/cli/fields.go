package cli

import (
	"fmt"
	"strings"
)

// fieldSpec is one parsed --fields entry with everything the generator
// templates need precomputed.
type fieldSpec struct {
	Name       string // snake_case column and JSON name
	Pascal     string // Go field name
	GoType     string // string, *time.Time, ...
	Kind       string // string|text|int|int64|float64|bool|time
	Nullable   bool
	SQLType    string // full column definition tail, e.g. "VARCHAR(255) NOT NULL"
	StructTag  string // complete struct tag including backticks
	FilterExpr string // query allowlist constructor, e.g. query.Partial("title")
	InputTag   string // extra validation constraints, e.g. "required,max=255"
	FakeExpr   string // factory value expression, e.g. f.Email()
}

const fieldsGrammar = `--fields "name:type,other:type?" with types string, text, int, int64, float64, bool, time (aliases: float, datetime, timestamp); a trailing ? makes the field nullable`

var reservedFieldNames = map[string]bool{"id": true, "created_at": true, "updated_at": true}

// parseFields parses the --fields DSL for the manifest's database dialect.
func parseFields(spec, database string) ([]fieldSpec, error) {
	if strings.TrimSpace(spec) == "" {
		spec = "name:string"
	}
	var fields []fieldSpec
	seen := map[string]bool{}
	for _, pair := range strings.Split(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name, kind, found := strings.Cut(pair, ":")
		if !found {
			return nil, fmt.Errorf("field %q must look like name:type (%s)", pair, fieldsGrammar)
		}
		name = strings.TrimSpace(name)
		kind = strings.TrimSpace(kind)
		nullable := strings.HasSuffix(kind, "?")
		kind = strings.TrimSuffix(kind, "?")
		if !isSnakeIdentifier(name) {
			return nil, fmt.Errorf("field name %q must be snake_case starting with a letter", name)
		}
		if reservedFieldNames[name] {
			return nil, fmt.Errorf("field %q is implicit: id, created_at, and updated_at are always generated", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("field %q is declared twice", name)
		}
		seen[name] = true
		field, err := buildField(name, kind, nullable, database)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("no fields declared (%s)", fieldsGrammar)
	}
	return fields, nil
}

// fakeExpression maps a field to a gofakeit expression, using the field name
// as a heuristic for realistic data. Nullable wrapping happens in the
// template through the emitted ptr helper.
func fakeExpression(field fieldSpec) string {
	name := field.Name
	contains := func(fragment string) bool { return strings.Contains(name, fragment) }
	switch field.Kind {
	case "string":
		switch {
		case contains("email"):
			return "f.Email()"
		case contains("name"):
			return "f.Name()"
		case contains("title"), contains("subject"):
			return `strings.TrimSuffix(f.Sentence(3), ".")`
		case contains("url"), contains("link"):
			return "f.URL()"
		case contains("phone"):
			return "f.Phone()"
		case contains("city"):
			return "f.City()"
		default:
			return `f.Seqf("` + name + ` %d") + " " + f.Word()`
		}
	case "text":
		return `f.Paragraph(1, 3, 12, " ")`
	case "int":
		return "f.Number(1, 1000)"
	case "int64":
		return "int64(f.Number(1, 100000))"
	case "float64":
		if contains("price") || contains("amount") || contains("cost") {
			return "f.Price(1, 1000)"
		}
		return "f.Float64Range(0, 1000)"
	case "bool":
		return "f.Bool()"
	default: // time
		return "f.PastDate().UTC()"
	}
}

func isSnakeIdentifier(name string) bool {
	if name == "" || !(name[0] >= 'a' && name[0] <= 'z') {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func buildField(name, kind string, nullable bool, database string) (fieldSpec, error) {
	switch kind {
	case "float":
		kind = "float64"
	case "datetime", "timestamp":
		kind = "time"
	}
	field := fieldSpec{Name: name, Pascal: pascalCase(name), Kind: kind, Nullable: nullable}
	gormSize := ""
	switch kind {
	case "string":
		field.GoType = "string"
		field.SQLType = "VARCHAR(255)"
		field.FilterExpr = fmt.Sprintf("query.Partial(%q)", name)
		field.InputTag = "required,max=255"
		gormSize = "size:255"
	case "text":
		field.GoType = "string"
		field.SQLType = "TEXT"
		field.FilterExpr = fmt.Sprintf("query.Partial(%q)", name)
		field.InputTag = "required"
	case "int":
		field.GoType = "int"
		field.SQLType = "INTEGER"
		field.FilterExpr = fmt.Sprintf("query.Compare(%q)", name)
	case "int64":
		field.GoType = "int64"
		field.SQLType = "BIGINT"
		field.FilterExpr = fmt.Sprintf("query.Compare(%q)", name)
	case "float64":
		field.GoType = "float64"
		switch database {
		case "postgres":
			field.SQLType = "DOUBLE PRECISION"
		case "sqlite":
			field.SQLType = "REAL"
		default:
			field.SQLType = "DOUBLE"
		}
		field.FilterExpr = fmt.Sprintf("query.Compare(%q)", name)
	case "bool":
		field.GoType = "bool"
		field.SQLType = "BOOLEAN"
		field.FilterExpr = fmt.Sprintf("query.Exact(%q).Bool()", name)
	case "time":
		field.GoType = "time.Time"
		field.SQLType = "TIMESTAMP"
		field.FilterExpr = fmt.Sprintf("query.Compare(%q)", name)
	default:
		return fieldSpec{}, fmt.Errorf("unknown field type %q (%s)", kind, fieldsGrammar)
	}
	if nullable {
		field.GoType = "*" + field.GoType
	} else if kind == "bool" {
		field.SQLType += " NOT NULL DEFAULT FALSE"
	} else {
		field.SQLType += " NOT NULL"
	}
	field.FakeExpr = fakeExpression(field)
	gorm := []string{}
	if gormSize != "" {
		gorm = append(gorm, gormSize)
	}
	if !nullable {
		gorm = append(gorm, "not null")
	}
	tag := fmt.Sprintf("json:%q db:%q", name, name)
	if len(gorm) > 0 {
		tag += fmt.Sprintf(" gorm:%q", strings.Join(gorm, ";"))
	}
	field.StructTag = "`" + tag + "`"
	return field, nil
}
