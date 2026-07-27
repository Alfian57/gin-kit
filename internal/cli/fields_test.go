package cli

import (
	"strings"
	"testing"
)

func TestParseFieldsHappyPath(t *testing.T) {
	fields, err := parseFields("title:string,body:text,count:int,total:int64,price:float,done:bool,due_at:datetime,notes:string?", "postgres")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]fieldSpec{}
	for _, field := range fields {
		byName[field.Name] = field
	}
	if byName["title"].GoType != "string" || !strings.Contains(byName["title"].SQLType, "VARCHAR(255) NOT NULL") {
		t.Fatalf("title: %+v", byName["title"])
	}
	if byName["price"].Kind != "float64" || byName["price"].SQLType != "DOUBLE PRECISION NOT NULL" {
		t.Fatalf("price alias/dialect: %+v", byName["price"])
	}
	if byName["done"].FilterExpr != `query.Exact("done").Bool()` || !strings.Contains(byName["done"].SQLType, "DEFAULT FALSE") {
		t.Fatalf("done: %+v", byName["done"])
	}
	if byName["due_at"].GoType != "time.Time" || byName["due_at"].Kind != "time" {
		t.Fatalf("due_at alias: %+v", byName["due_at"])
	}
	if byName["notes"].GoType != "*string" || strings.Contains(byName["notes"].SQLType, "NOT NULL") {
		t.Fatalf("nullable: %+v", byName["notes"])
	}
}

func TestParseFieldsValidationTagsRespectNullability(t *testing.T) {
	fields, err := parseFields("nickname:string?,bio:text?,password:string", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]fieldSpec{}
	for _, field := range fields {
		byName[field.Name] = field
	}
	if byName["nickname"].InputTag != "omitempty,max=255" {
		t.Fatalf("nullable string must not be required: %+v", byName["nickname"])
	}
	if byName["bio"].InputTag != "" {
		t.Fatalf("nullable text must carry no constraints: %+v", byName["bio"])
	}
	if byName["password"].InputTag != "required,max=255" || !byName["password"].Sensitive {
		t.Fatalf("password: %+v", byName["password"])
	}
	if byName["nickname"].Sensitive || byName["bio"].Sensitive {
		t.Fatal("non-credential fields flagged sensitive")
	}
}

func TestFakeExpressionForeignKeys(t *testing.T) {
	fields, err := parseFields("user_id:string,parent_id:string?,email:string,city_name:string", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]fieldSpec{}
	for _, field := range fields {
		byName[field.Name] = field
	}
	if byName["user_id"].FakeExpr != "f.UUID()" {
		t.Fatalf("user_id must fake as a UUID: %+v", byName["user_id"])
	}
	if byName["parent_id"].FakeExpr != "f.UUID()" {
		t.Fatalf("parent_id must fake as a UUID: %+v", byName["parent_id"])
	}
	if byName["email"].FakeExpr != "f.Email()" {
		t.Fatalf("email heuristic regressed: %+v", byName["email"])
	}
	if byName["city_name"].FakeExpr != "f.Name()" {
		t.Fatalf("name heuristic regressed: %+v", byName["city_name"])
	}
}

func TestParseFieldsDialects(t *testing.T) {
	for database, want := range map[string]string{
		"postgres": "DOUBLE PRECISION",
		"mysql":    "DOUBLE",
		"mariadb":  "DOUBLE",
		"sqlite":   "REAL",
	} {
		fields, err := parseFields("price:float64", database)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(fields[0].SQLType, want) {
			t.Errorf("%s: %q", database, fields[0].SQLType)
		}
	}
}

func TestParseFieldsDefaultsToName(t *testing.T) {
	fields, err := parseFields("", "sqlite")
	if err != nil || len(fields) != 1 || fields[0].Name != "name" {
		t.Fatalf("default fields: %+v err=%v", fields, err)
	}
}

func TestParseFieldsRejections(t *testing.T) {
	for name, spec := range map[string]string{
		"missing type":     "title",
		"unknown type":     "title:uuid",
		"reserved id":      "id:string",
		"reserved created": "created_at:time",
		"reserved deleted": "deleted_at:time",
		"duplicate":        "title:string,title:text",
		"bad name":         "Title:string",
		"empty":            ",",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseFields(spec, "sqlite"); err == nil {
				t.Fatalf("spec %q accepted", spec)
			}
		})
	}
}
