package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"testing"
)

// parseDoccheckFixture parses one in-memory file used to exercise AST rules.
func parseDoccheckFixture(t *testing.T, source string) (*token.FileSet, *ast.File) {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "fixture.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return fileSet, file
}

// fixtureChecker builds the minimal state needed to inspect one parsed source.
func fixtureChecker(fileSet *token.FileSet, file *ast.File) *checker {
	return &checker{
		Files:    map[string][]*ast.File{"fixture": {file}},
		FileSets: map[*ast.File]*token.FileSet{file: fileSet},
		Paths:    map[*ast.File]string{file: "fixture.go"},
	}
}

func TestHasPackageComment(t *testing.T) {
	_, documented := parseDoccheckFixture(t, "// Package fixture documents this package.\npackage fixture\n")
	_, undocumented := parseDoccheckFixture(t, "package fixture\n")
	if !hasPackageComment([]*ast.File{undocumented, documented}) {
		t.Fatal("documented package was rejected")
	}
	if hasPackageComment([]*ast.File{undocumented}) {
		t.Fatal("undocumented package was accepted")
	}
}

func TestInspectDeclarationFindsReusableTestHelpers(t *testing.T) {
	fileSet, file := parseDoccheckFixture(t, `package fixture
type sample struct { Value string }
type contract interface { Execute() }
func helper() {}
func TestCase() {}
`)
	check := fixtureChecker(fileSet, file)
	for _, declaration := range file.Decls {
		check.inspectDeclaration(file, declaration, true)
	}
	actual := make([]string, 0, len(check.Violations))
	for _, item := range check.Violations {
		actual = append(actual, item.Name)
	}
	sort.Strings(actual)
	want := []string{"Execute", "Value", "contract", "helper", "sample"}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("violations = %v, want %v", actual, want)
	}
}

func TestInspectDeclarationAcceptsDocumentedContracts(t *testing.T) {
	fileSet, file := parseDoccheckFixture(t, `package fixture
// sample stores a fixture value.
type sample struct {
	// Value is the fixture's text value.
	Value string
}
// contract defines the fixture operation.
type contract interface {
	// Execute performs the fixture operation.
	Execute()
}
// helper builds a fixture value.
func helper() {}
`)
	check := fixtureChecker(fileSet, file)
	for _, declaration := range file.Decls {
		check.inspectDeclaration(file, declaration, false)
	}
	if len(check.Violations) != 0 {
		t.Fatalf("documented declarations produced violations: %+v", check.Violations)
	}
}
