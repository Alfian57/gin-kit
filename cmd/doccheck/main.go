// Command doccheck verifies gin-kit source documentation conventions.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// sourceRoots contains every repository directory whose Go declarations are
// maintained as framework or CLI implementation code.
var sourceRoots = []string{"cmd", "internal", "runtime"}

// violation describes one declaration that lacks its required doc comment.
type violation struct {
	// Path is the repository-relative source file containing the declaration.
	Path string
	// Line is the one-based source line at which the declaration starts.
	Line int
	// Name identifies the package, declaration, field, or method to document.
	Name string
}

// checker accumulates documentation violations while scanning one repository.
type checker struct {
	// Root is the absolute repository directory used to resolve source roots.
	Root string
	// Files maps package directories to their parsed non-test production files.
	Files map[string][]*ast.File
	// FileSets maps parsed source files to the token metadata used for lines.
	FileSets map[*ast.File]*token.FileSet
	// Paths maps parsed source files to their repository-relative paths.
	Paths map[*ast.File]string
	// Violations collects every missing comment in deterministic output order.
	Violations []violation
}

// main scans the repository and exits non-zero when a required comment is
// absent, making the command suitable for local use and continuous integration.
func main() {
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	check := checker{
		Root:     root,
		Files:    make(map[string][]*ast.File),
		FileSets: make(map[*ast.File]*token.FileSet),
		Paths:    make(map[*ast.File]string),
	}
	if err := check.scan(); err != nil {
		fail(err)
	}
	check.inspect()
	if len(check.Violations) == 0 {
		return
	}
	sort.Slice(check.Violations, func(i, j int) bool {
		left, right := check.Violations[i], check.Violations[j]
		if left.Path == right.Path {
			if left.Line == right.Line {
				return left.Name < right.Name
			}
			return left.Line < right.Line
		}
		return left.Path < right.Path
	})
	for _, item := range check.Violations {
		fmt.Fprintf(os.Stderr, "%s:%d: missing documentation for %s\n", item.Path, item.Line, item.Name)
	}
	fmt.Fprintln(os.Stderr, "document every package, declaration, struct field, and interface method before continuing")
	os.Exit(1)
}

// scan parses every Go source file below the configured repository roots.
func (c *checker) scan() error {
	for _, root := range sourceRoots {
		if err := filepath.WalkDir(filepath.Join(c.Root, root), c.parseFile); err != nil {
			return err
		}
	}
	return nil
}

// parseFile records one parseable Go source file and groups production files
// by directory so package comments can be checked once per package.
func (c *checker) parseFile(path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if entry.IsDir() || !strings.HasSuffix(path, ".go") {
		return nil
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(c.Root, path)
	if err != nil {
		return err
	}
	c.FileSets[file] = fileSet
	c.Paths[file] = filepath.ToSlash(relative)
	if !strings.HasSuffix(path, "_test.go") {
		c.Files[filepath.Dir(path)] = append(c.Files[filepath.Dir(path)], file)
	}
	return nil
}

// inspect verifies package comments, all production declarations, and only
// reusable helpers and fixtures in test files.
func (c *checker) inspect() {
	for _, files := range c.Files {
		if !hasPackageComment(files) {
			c.add(files[0], files[0].Package, "package "+files[0].Name.Name)
		}
	}
	for file, path := range c.Paths {
		isTest := strings.HasSuffix(path, "_test.go")
		for _, declaration := range file.Decls {
			c.inspectDeclaration(file, declaration, isTest)
		}
	}
}

// inspectDeclaration checks a package-level declaration and recursively checks
// the fields and interface methods that make a type's contract understandable.
func (c *checker) inspectDeclaration(file *ast.File, declaration ast.Decl, isTest bool) {
	switch item := declaration.(type) {
	case *ast.FuncDecl:
		if !isTest || !isTestCase(item.Name.Name) {
			c.require(file, item.Doc, item.Pos(), item.Name.Name)
		}
	case *ast.GenDecl:
		for _, specification := range item.Specs {
			switch value := specification.(type) {
			case *ast.TypeSpec:
				c.require(file, commentFor(value.Doc, item.Doc), value.Pos(), value.Name.Name)
				c.inspectType(file, value.Type)
			case *ast.ValueSpec:
				if isTest && allTestCaseNames(value.Names) {
					continue
				}
				for _, name := range value.Names {
					c.require(file, commentFor(value.Doc, item.Doc), name.Pos(), name.Name)
				}
			}
		}
	}
}

// inspectType checks named fields in structs and named methods in interfaces.
func (c *checker) inspectType(file *ast.File, expression ast.Expr) {
	switch value := expression.(type) {
	case *ast.StructType:
		for _, field := range value.Fields.List {
			for _, name := range field.Names {
				c.require(file, commentFor(field.Doc, field.Comment), name.Pos(), name.Name)
			}
		}
	case *ast.InterfaceType:
		for _, method := range value.Methods.List {
			for _, name := range method.Names {
				c.require(file, commentFor(method.Doc, method.Comment), name.Pos(), name.Name)
			}
		}
	}
}

// require records a violation when a declaration has no adjacent comment.
func (c *checker) require(file *ast.File, group *ast.CommentGroup, position token.Pos, name string) {
	if group != nil && strings.TrimSpace(group.Text()) != "" {
		return
	}
	c.add(file, position, name)
}

// add records a repository-relative violation for one token position.
func (c *checker) add(file *ast.File, position token.Pos, name string) {
	c.Violations = append(c.Violations, violation{
		Path: c.Paths[file],
		Line: c.FileSets[file].Position(position).Line,
		Name: name,
	})
}

// hasPackageComment reports whether any production file documents its package.
func hasPackageComment(files []*ast.File) bool {
	for _, file := range files {
		if file.Doc == nil {
			continue
		}
		text := strings.TrimSpace(file.Doc.Text())
		prefix := "Package " + file.Name.Name
		if strings.HasPrefix(text, prefix) || file.Name.Name == "main" && strings.HasPrefix(text, "Command ") {
			return true
		}
	}
	return false
}

// commentFor returns the first non-nil adjacent documentation comment.
func commentFor(groups ...*ast.CommentGroup) *ast.CommentGroup {
	for _, group := range groups {
		if group != nil && strings.TrimSpace(group.Text()) != "" {
			return group
		}
	}
	return nil
}

// isTestCase identifies test entry points whose names already describe their
// purpose and therefore do not need a redundant doc comment.
func isTestCase(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Example", "Fuzz"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// allTestCaseNames reports whether every declaration name is a test entry point.
func allTestCaseNames(names []*ast.Ident) bool {
	for _, name := range names {
		if !isTestCase(name.Name) {
			return false
		}
	}
	return len(names) > 0
}

// fail prints one setup error and terminates the command with a failure status.
func fail(err error) {
	fmt.Fprintln(os.Stderr, "doccheck:", err)
	os.Exit(1)
}
