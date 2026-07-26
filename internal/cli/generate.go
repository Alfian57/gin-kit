package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

//go:embed all:generators
var generatorFS embed.FS

// genData is the template context for every generator.
type genData struct {
	Manifest
	Name           string // Ticket
	Camel          string // ticket
	Snake          string // ticket
	Plural         string // Tickets
	PluralCamel    string // tickets (route segment)
	Table          string // tickets
	Fields         []fieldSpec
	HasTime        bool        // any user field is time-typed
	StringFields   []fieldSpec // required string/text fields (service validation)
	SampleJSON     string      // JSON body with valid sample values for tests
	QueryImport    string      // edition-specific query package import path
	DatabaseImport string      // edition-specific database package import path

	// Precomputed SQL fragments for the sqlx repository variant.
	ColumnList         string // id, title, ..., created_at, updated_at
	InsertPlaceholders string // ?, ?, ...
	InsertArgs         string // entity.ID, entity.Title, ...
	UpdateSet          string // title = ?, ..., updated_at = ?
	UpdateArgs         string // entity.Title, ..., entity.UpdatedAt, entity.ID
	FilterList         string // query.Partial("title"), ...
	SortList           string // query.SortBy("created_at"), ...
}

func buildGenData(m Manifest, name, fieldsSpec, table string) (genData, error) {
	pascal, camel, snake, err := validateGeneratorName(name)
	if err != nil {
		return genData{}, err
	}
	fields, err := parseFields(fieldsSpec, m.Database)
	if err != nil {
		return genData{}, err
	}
	if table == "" {
		table = pluralize(snake)
	}
	data := genData{
		Manifest:    m,
		Name:        pascal,
		Camel:       camel,
		Snake:       snake,
		Plural:      pluralize(pascal),
		PluralCamel: pluralize(camel),
		Table:       table,
		Fields:      fields,
	}
	if m.Edition == "framework" {
		data.QueryImport = "github.com/Alfian57/gin-kit/framework/query"
		data.DatabaseImport = "github.com/Alfian57/gin-kit/framework/database"
	} else {
		data.QueryImport = m.Module + "/internal/platform/query"
		data.DatabaseImport = m.Module + "/internal/platform/database"
	}
	columns := []string{"id"}
	insertArgs := []string{"entity.ID"}
	var updateSet, updateArgs, filters, sorts, samples []string
	sorts = append(sorts, `query.SortBy("created_at")`)
	for _, field := range fields {
		if strings.TrimPrefix(field.GoType, "*") == "time.Time" {
			data.HasTime = true
		}
		if (field.Kind == "string" || field.Kind == "text") && !field.Nullable {
			data.StringFields = append(data.StringFields, field)
		}
		columns = append(columns, field.Name)
		insertArgs = append(insertArgs, "entity."+field.Pascal)
		updateSet = append(updateSet, field.Name+" = ?")
		updateArgs = append(updateArgs, "entity."+field.Pascal)
		filters = append(filters, field.FilterExpr)
		sorts = append(sorts, fmt.Sprintf("query.SortBy(%q)", field.Name))
		samples = append(samples, fmt.Sprintf("%q: %s", field.Name, sampleJSONValue(field)))
	}
	data.SampleJSON = "{" + strings.Join(samples, ", ") + "}"
	columns = append(columns, "created_at", "updated_at")
	insertArgs = append(insertArgs, "entity.CreatedAt", "entity.UpdatedAt")
	updateSet = append(updateSet, "updated_at = ?")
	updateArgs = append(updateArgs, "entity.UpdatedAt", "entity.ID")
	data.ColumnList = strings.Join(columns, ", ")
	data.InsertPlaceholders = strings.TrimSuffix(strings.Repeat("?, ", len(columns)), ", ")
	data.InsertArgs = strings.Join(insertArgs, ", ")
	data.UpdateSet = strings.Join(updateSet, ", ")
	data.UpdateArgs = strings.Join(updateArgs, ", ")
	data.FilterList = strings.Join(filters, ", ")
	data.SortList = strings.Join(sorts, ", ")
	return data, nil
}

func sampleJSONValue(field fieldSpec) string {
	switch field.Kind {
	case "string":
		return `"Sample value"`
	case "text":
		return `"Sample body text"`
	case "int", "int64":
		return "7"
	case "float64":
		return "9.5"
	case "bool":
		return "true"
	default: // time
		return `"2026-01-02T15:04:05Z"`
	}
}

func renderGenerator(templatePath string, data genData) ([]byte, error) {
	raw, err := generatorFS.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("generator template %s: %w", templatePath, err)
	}
	parser := template.New(filepath.Base(templatePath))
	// HTML output keeps {{...}} for runtime rendering; the generator itself
	// uses [[...]] delimiters there.
	if strings.HasSuffix(templatePath, ".html.tmpl") {
		parser = parser.Delims("[[", "]]")
	}
	parsed, err := parser.Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("generator template %s: %w", templatePath, err)
	}
	var output strings.Builder
	if err := parsed.Execute(&output, data); err != nil {
		return nil, fmt.Errorf("generator template %s: %w", templatePath, err)
	}
	return []byte(output.String()), nil
}

// generateRequest describes one generator invocation.
type generateRequest struct {
	Kind   string
	Name   string
	Fields string
	Table  string
}

// runGenerate resolves a request into absolute file paths plus a next-steps
// message. It performs no writes, which makes it the unit-test seam.
func runGenerate(rootDir string, m Manifest, request generateRequest) (map[string][]byte, string, error) {
	data, err := buildGenData(m, request.Name, request.Fields, request.Table)
	if err != nil {
		return nil, "", diagnostic("generator_input_invalid", "generate "+request.Kind, request.Name, err, "Fix the generator input and retry: "+fieldsGrammar)
	}
	edition := "starter"
	if m.Edition == "framework" {
		edition = "framework"
	}
	handlerDir, handlerPackage := "api", "api"
	if m.Mode == "ui" {
		handlerDir, handlerPackage = "web", "web"
	}
	files := map[string][]byte{}
	add := func(rel, templatePath string) error {
		content, err := renderGenerator(templatePath, data)
		if err != nil {
			return err
		}
		files[filepath.Join(rootDir, rel)] = content
		return nil
	}
	var nextSteps string

	switch request.Kind {
	case "resource":
		steps := []struct{ rel, tmpl string }{
			{filepath.Join("internal", "domain", data.Snake+".go"), "generators/resource/shared/domain.go.tmpl"},
			{filepath.Join("internal", "repository", data.Snake+"_repository.go"), "generators/resource/shared/repository.go.tmpl"},
			{filepath.Join("internal", "service", data.Snake+"_service.go"), "generators/resource/shared/service.go.tmpl"},
			{filepath.Join("internal", "service", data.Snake+"_service_test.go"), "generators/resource/shared/service_test.go.tmpl"},
		}
		if m.Edition == "starter" && m.Mode == "ui" {
			steps = append(steps,
				struct{ rel, tmpl string }{filepath.Join("internal", "handler", "web", data.Snake+"_handler.go"), "generators/resource/starter/handler_web.go.tmpl"},
				struct{ rel, tmpl string }{filepath.Join("web", "templates", data.PluralCamel+".html"), "generators/resource/starter/web_template.html.tmpl"},
			)
		} else {
			steps = append(steps,
				struct{ rel, tmpl string }{filepath.Join("internal", "handler", handlerDir, data.Snake+"_handler.go"), "generators/resource/" + edition + "/handler_api.go.tmpl"},
				struct{ rel, tmpl string }{filepath.Join("internal", "handler", handlerDir, data.Snake+"_handler_test.go"), "generators/resource/" + edition + "/handler_api_test.go.tmpl"},
			)
		}
		for _, step := range steps {
			if err := add(step.rel, step.tmpl); err != nil {
				return nil, "", err
			}
		}
		migration, err := renderGenerator("generators/resource/migration.sql.tmpl", data)
		if err != nil {
			return nil, "", err
		}
		files[migrationPath(rootDir, "create_"+data.Table)] = migration
		nextSteps = resourceNextSteps(m, data, handlerPackage)
	case "domain":
		if err := add(filepath.Join("internal", "domain", data.Snake+".go"), "generators/resource/shared/domain.go.tmpl"); err != nil {
			return nil, "", err
		}
		nextSteps = fmt.Sprintf("Generated the %s model and %sRepository interface. Pair it with:\n  gin-kit generate repository %s --fields %q", data.Name, data.Name, data.Name, request.Fields)
	case "repository":
		if err := add(filepath.Join("internal", "repository", data.Snake+"_repository.go"), "generators/resource/shared/repository.go.tmpl"); err != nil {
			return nil, "", err
		}
		nextSteps = fmt.Sprintf("The repository implements domain.%sRepository — generate the domain first if it does not exist:\n  gin-kit generate domain %s --fields %q", data.Name, data.Name, request.Fields)
	case "service":
		if err := add(filepath.Join("internal", "service", data.Snake+"_service.go"), "generators/single/service.go.tmpl"); err != nil {
			return nil, "", err
		}
	case "handler":
		if err := add(filepath.Join("internal", "handler", handlerDir, data.Snake+"_handler.go"), "generators/single/handler_"+handlerPackage+".go.tmpl"); err != nil {
			return nil, "", err
		}
		nextSteps = fmt.Sprintf("Wire the routes where %s.Register is called:\n  %s.Register%sRoutes(...)", handlerPackage, handlerPackage, data.Plural)
	case "middleware":
		if err := add(filepath.Join("internal", "middleware", data.Snake+".go"), "generators/single/middleware.go.tmpl"); err != nil {
			return nil, "", err
		}
		if m.Edition == "framework" {
			nextSteps = fmt.Sprintf("Install it in internal/app/app.go:\n  application.Use(middleware.%s())", data.Name)
		} else {
			nextSteps = fmt.Sprintf("Install it in internal/app/app.go:\n  r.Use(middleware.%s())", data.Name)
		}
	default:
		return nil, "", diagnostic("generator_unknown", "generate", request.Kind, fmt.Errorf("unknown generator %q", request.Kind), "Run gin-kit generate --help for the available generators.")
	}
	return files, nextSteps, nil
}

func migrationPath(rootDir, name string) string {
	stamp := time.Now().UTC().Format("20060102150405")
	path := filepath.Join(rootDir, "migrations", fmt.Sprintf("%s_%s.sql", stamp, name))
	for i := 1; ; i++ {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			break
		}
		path = filepath.Join(rootDir, "migrations", fmt.Sprintf("%s_%s_%d.sql", stamp, name, i))
	}
	return path
}

func resourceNextSteps(m Manifest, data genData, handlerPackage string) string {
	var wiring string
	if m.Edition == "framework" {
		wiring = fmt.Sprintf(`In internal/app/app.go, after the existing Register call, add:

    %s := service.New%sService(repository.New%sRepository(application.Database()))
    %s.Register%s(router, %s)

and import "%s/internal/repository" and "%s/internal/service".`,
			data.PluralCamel, data.Name, data.Name,
			handlerPackage, data.Plural, data.PluralCamel,
			m.Module, m.Module)
	} else {
		wiring = fmt.Sprintf(`In internal/app/app.go, after the existing Register call, add:

    %s.Register%s(r, service.New%sService(repository.New%sRepository(db)))

and import "%s/internal/repository" and "%s/internal/service".`,
			handlerPackage, data.Plural, data.Name, data.Name,
			m.Module, m.Module)
	}
	return fmt.Sprintf(`Routes are not registered automatically (explicit wiring).

%s

Then apply the migration:

    gin-kit db up`, wiring)
}

func generateCommand() *cobra.Command {
	var dryRun bool
	generate := &cobra.Command{Use: "generate", Short: "Generate project building blocks from real, manifest-aware templates"}
	generate.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show generated files without writing them")

	runKind := func(kind string, fields, table *string) func(*cobra.Command, []string) error {
		return func(_ *cobra.Command, args []string) error {
			rootDir, m, err := projectRoot()
			if err != nil {
				return err
			}
			request := generateRequest{Kind: kind, Name: args[0]}
			if fields != nil {
				request.Fields = *fields
			}
			if table != nil {
				request.Table = *table
			}
			files, nextSteps, err := runGenerate(rootDir, m, request)
			if err != nil {
				return err
			}
			if err := writeGeneratedFiles(rootDir, files, dryRun); err != nil {
				return err
			}
			if nextSteps != "" {
				fmt.Println("\n" + nextSteps)
			}
			return nil
		}
	}

	resource := &cobra.Command{
		Use:   "resource <Name>",
		Short: "Generate a full vertical slice: model, repository, service, handler, tests, and migration",
		Args:  cobra.ExactArgs(1),
	}
	resourceFields := resource.Flags().String("fields", "", fieldsGrammar)
	resourceTable := resource.Flags().String("table", "", "override the derived table name")
	resource.RunE = runKind("resource", resourceFields, resourceTable)
	generate.AddCommand(resource)

	domain := &cobra.Command{
		Use:   "domain <Name>",
		Short: "Generate a domain model with its repository interface",
		Args:  cobra.ExactArgs(1),
	}
	domainFields := domain.Flags().String("fields", "", fieldsGrammar)
	domain.RunE = runKind("domain", domainFields, nil)
	generate.AddCommand(domain)

	repository := &cobra.Command{
		Use:   "repository <Name>",
		Short: "Generate a repository implementation for an existing domain model",
		Args:  cobra.ExactArgs(1),
	}
	repositoryFields := repository.Flags().String("fields", "", fieldsGrammar+" (must match the domain model)")
	repository.RunE = runKind("repository", repositoryFields, nil)
	generate.AddCommand(repository)

	for _, kind := range []string{"service", "handler", "middleware"} {
		k := kind
		generate.AddCommand(&cobra.Command{
			Use:   k + " <Name>",
			Short: "Generate a " + k,
			Args:  cobra.ExactArgs(1),
			RunE:  runKind(k, nil, nil),
		})
	}

	generate.AddCommand(&cobra.Command{
		Use:   "migration <name>",
		Short: "Generate an empty timestamped goose migration",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			rootDir, _, err := projectRoot()
			if err != nil {
				return err
			}
			path := migrationPath(rootDir, snakeCase(args[0]))
			return writeGeneratedFiles(rootDir, map[string][]byte{path: []byte("-- +goose Up\n\n-- +goose Down\n")}, dryRun)
		},
	})
	return generate
}
