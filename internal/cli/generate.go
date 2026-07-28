package cli

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

// generatorFS define package-level implementation state.
//
//go:embed all:generators
var generatorFS embed.FS

// genData is the template context for every generator.
type genData struct {
	Manifest
	Name        string // Ticket
	Camel       string // ticket
	Snake       string // ticket
	Plural      string // Tickets
	PluralCamel string // tickets (route segment)
	Table       string // tickets
	// Fields store data used by this type.
	Fields         []fieldSpec
	HasTime        bool        // any user field is time-typed
	StringFields   []fieldSpec // required string/text fields (service validation)
	SampleJSON     string      // JSON body with valid sample values for tests
	QueryImport    string      // project-type-specific query package import path
	AuthzImport    string      // project-type-specific authz package import path
	DatabaseImport string      // project-type-specific database package import path
	FactoryImport  string      // project-type-specific factory package import path
	HasNullable    bool        // any nullable field (factory ptr helper)
	HasStringFake  bool        // any FakeExpr using the strings package
	HasSensitive   bool        // any credential-like field (kept out of responses)
	SensitiveNames string      // comma-separated names of excluded fields
	SQLiteSchema   string      // CREATE TABLE statement for sqlite-backed integration tests
	SoftDelete     bool        // --soft-delete: deleted_at column plus explicitly filtered queries

	// Precomputed SQL fragments for the sqlx repository variant.
	ColumnList         string // id, title, ..., created_at, updated_at
	InsertPlaceholders string // ?, ?, ...
	InsertArgs         string // entity.ID, entity.Title, ...
	UpdateSet          string // title = ?, ..., updated_at = ?
	UpdateArgs         string // entity.Title, ..., entity.UpdatedAt, entity.ID
	FilterList         string // query.Partial("title"), ...
	SortList           string // query.SortBy("created_at"), ...
}

// buildGenData performs this package operation.
func buildGenData(m Manifest, name, fieldsSpec, table string, softDelete bool) (genData, error) {
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
		SoftDelete:  softDelete,
	}
	if m.ProjectType == "runtime" {
		data.QueryImport = "github.com/Alfian57/gin-kit/runtime/query"
		data.AuthzImport = "github.com/Alfian57/gin-kit/runtime/authz"
		data.DatabaseImport = "github.com/Alfian57/gin-kit/runtime/database"
		data.FactoryImport = "github.com/Alfian57/gin-kit/runtime/factory"
	} else {
		data.QueryImport = m.Module + "/internal/platform/query"
		data.AuthzImport = m.Module + "/internal/platform/authz"
		data.DatabaseImport = m.Module + "/internal/platform/database"
		data.FactoryImport = m.Module + "/internal/platform/factory"
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
		if field.Nullable {
			data.HasNullable = true
		}
		if strings.Contains(field.FakeExpr, "strings.") {
			data.HasStringFake = true
		}
		if field.Sensitive {
			data.HasSensitive = true
			if data.SensitiveNames != "" {
				data.SensitiveNames += ", "
			}
			data.SensitiveNames += field.Name
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
	var schemaColumns []string
	schemaColumns = append(schemaColumns, "id VARCHAR(36) PRIMARY KEY")
	for _, field := range fields {
		schemaColumns = append(schemaColumns, field.Name+" "+sqliteColumnType(field))
	}
	schemaColumns = append(schemaColumns, "created_at TIMESTAMP NOT NULL", "updated_at TIMESTAMP NOT NULL")
	if softDelete {
		schemaColumns = append(schemaColumns, "deleted_at TIMESTAMP")
	}
	data.SQLiteSchema = "CREATE TABLE IF NOT EXISTS " + table + " (" + strings.Join(schemaColumns, ", ") + ")"
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

// sampleJSONValue performs this package operation.
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

// renderGenerator performs this package operation.
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
	// Kind store data used by this type.
	Kind string
	// Name store data used by this type.
	Name string
	// Fields store data used by this type.
	Fields string
	// Table store data used by this type.
	Table string
	// SoftDelete store data used by this type.
	SoftDelete bool
}

// runGenerate resolves a request into absolute file paths plus a next-steps
// message. It performs no writes, which makes it the unit-test seam.
func runGenerate(rootDir string, m Manifest, request generateRequest) (map[string][]byte, string, error) {
	data, err := buildGenData(m, request.Name, request.Fields, request.Table, request.SoftDelete)
	if err != nil {
		return nil, "", diagnostic("generator_input_invalid", "generate "+request.Kind, request.Name, err, "Fix the generator input and retry: "+fieldsGrammar)
	}
	projectType := "standalone"
	if m.ProjectType == "runtime" {
		projectType = "runtime"
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
			{filepath.Join("internal", "dto", data.Snake+"_dto.go"), "generators/resource/shared/dto.go.tmpl"},
			{filepath.Join("internal", "repository", data.Snake+"_repository.go"), "generators/resource/shared/repository.go.tmpl"},
			{filepath.Join("internal", "service", data.Snake+"_service.go"), "generators/resource/shared/service.go.tmpl"},
			{filepath.Join("internal", "service", data.Snake+"_service_test.go"), "generators/resource/shared/service_test.go.tmpl"},
			{filepath.Join("internal", "database", "factories", data.Snake+"_factory.go"), "generators/single/factory.go.tmpl"},
		}
		if m.ProjectType == "runtime" {
			// The repository integration test relies on runtime/apptest,
			// which standalone projects cannot import.
			steps = append(steps, struct{ rel, tmpl string }{
				filepath.Join("internal", "repository", data.Snake+"_repository_test.go"),
				"generators/resource/runtime/repository_test.go.tmpl",
			})
		}
		if m.ProjectType == "standalone" && m.Mode == "ui" {
			steps = append(steps,
				struct{ rel, tmpl string }{filepath.Join("internal", "handler", "web", data.Snake+"_handler.go"), "generators/resource/standalone/handler_web.go.tmpl"},
				struct{ rel, tmpl string }{filepath.Join("web", "templates", data.PluralCamel+".html"), "generators/resource/standalone/web_template.html.tmpl"},
			)
		} else {
			steps = append(steps,
				struct{ rel, tmpl string }{filepath.Join("internal", "handler", handlerDir, data.Snake+"_handler.go"), "generators/resource/" + projectType + "/handler_api.go.tmpl"},
				struct{ rel, tmpl string }{filepath.Join("internal", "handler", handlerDir, data.Snake+"_handler_test.go"), "generators/resource/" + projectType + "/handler_api_test.go.tmpl"},
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
	case "dto":
		if err := add(filepath.Join("internal", "dto", data.Snake+"_dto.go"), "generators/resource/shared/dto.go.tmpl"); err != nil {
			return nil, "", err
		}
		nextSteps = fmt.Sprintf("Generated Create%sRequest, Update%sRequest, and %sResponse. The response mapper needs the domain model:\n  gin-kit generate domain %s --fields %q", data.Name, data.Name, data.Name, data.Name, request.Fields)
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
	case "job", "event", "mail":
		if m.ProjectType != "runtime" {
			return nil, "", diagnostic("generator_project_type_unsupported", "generate "+request.Kind, m.ProjectType,
				fmt.Errorf("the %s generator relies on gin-kit runtime packages", request.Kind),
				"Use a Runtime project, or write a standalone implementation by hand.")
		}
		switch request.Kind {
		case "job":
			if err := add(filepath.Join("internal", "jobs", data.Snake+".go"), "generators/runtime-only/job.go.tmpl"); err != nil {
				return nil, "", err
			}
			nextSteps = fmt.Sprintf("Register the handler inside func Register in internal/jobs:\n\n    queue.Register(q, Type%s, Handle%s)\n\nDispatch it with:\n\n    queue.Dispatch(ctx, application.Queue(), jobs.Type%s, jobs.%sPayload{...})", data.Name, data.Name, data.Name, data.Name)
		case "event":
			if err := add(filepath.Join("internal", "event", data.Snake+".go"), "generators/runtime-only/event.go.tmpl"); err != nil {
				return nil, "", err
			}
			nextSteps = fmt.Sprintf("Create a bus during wiring and subscribe the listener:\n\n    bus := events.NewBus()\n    events.On(bus, event.On%s)\n\nEmit it where the change happens:\n\n    err := events.Emit(ctx, bus, event.%s{...})", data.Name, data.Name)
		case "mail":
			if err := add(filepath.Join("internal", "mail", data.Snake+".go"), "generators/runtime-only/mail.go.tmpl"); err != nil {
				return nil, "", err
			}
			if err := add(filepath.Join("web", "templates", "mail", data.Snake+".html"), "generators/runtime-only/mail_template.html.tmpl"); err != nil {
				return nil, "", err
			}
			nextSteps = fmt.Sprintf("Build the mailer from configuration and send:\n\n    mailer, err := mail.New(cfg.MailOptions())\n    templates := template.Must(template.ParseGlob(\"web/templates/mail/*.html\"))\n    err = mailer.Send(ctx, projectmail.New%s(templates, recipient))", data.Name)
		}
	case "factory":
		if err := add(filepath.Join("internal", "database", "factories", data.Snake+"_factory.go"), "generators/single/factory.go.tmpl"); err != nil {
			return nil, "", err
		}
		nextSteps = fmt.Sprintf("Use the factory in tests and seeders:\n\n    item := factories.New%sFactory().Make()\n    persisted, err := factories.New%sFactory().Create(ctx, repo.Create)", data.Name, data.Name)
	case "seeder":
		if err := add(filepath.Join("internal", "database", "seeders", data.Snake+".go"), "generators/single/seeder.go.tmpl"); err != nil {
			return nil, "", err
		}
		nextSteps = fmt.Sprintf("Register the seeder in internal/database/seeders/seeders.go inside All():\n\n    {Name: %q, Run: Seed%s},\n\nThen run it with:\n\n    gin-kit db seed", data.Snake, data.Name)
	case "policy":
		if err := add(filepath.Join("internal", "policy", data.Snake+"_policy.go"), "generators/single/policy.go.tmpl"); err != nil {
			return nil, "", err
		}
		if err := add(filepath.Join("internal", "policy", data.Snake+"_policy_test.go"), "generators/single/policy_test.go.tmpl"); err != nil {
			return nil, "", err
		}
		if m.ProjectType == "standalone" {
			// Back-fill the vendored authz package for standalone projects
			// scaffolded before it existed.
			if _, statErr := os.Stat(filepath.Join(rootDir, "internal", "platform", "authz", "authz.go")); os.IsNotExist(statErr) {
				if err := add(filepath.Join("internal", "platform", "authz", "authz.go"), "generators/single/platform_authz.go.tmpl"); err != nil {
					return nil, "", err
				}
			}
		}
		subjectExpr := "auth.UserID(c)"
		if m.ProjectType != "runtime" {
			subjectExpr = `c.GetString("user_id")`
		}
		nextSteps = fmt.Sprintf(`Enforce the policy in the handler before acting (explicit wiring):

    p := policy.New%sPolicy()
    if !authz.Authorize(c, p.CanUpdate(c.Request.Context(), %s, entity)) {
        return
    }

Import %q and %q.
The subject %s is empty for unauthenticated requests, which the
generated placeholder rules deny. The policy references domain.%s — run
gin-kit generate domain %s first if it does not exist.`,
			data.Name, subjectExpr,
			data.AuthzImport, m.Module+"/internal/policy",
			subjectExpr, data.Name, data.Name)
	case "middleware":
		if err := add(filepath.Join("internal", "middleware", data.Snake+".go"), "generators/single/middleware.go.tmpl"); err != nil {
			return nil, "", err
		}
		if m.ProjectType == "runtime" {
			nextSteps = fmt.Sprintf("Install it in internal/app/app.go:\n  application.Use(middleware.%s())", data.Name)
		} else {
			nextSteps = fmt.Sprintf("Install it in internal/app/app.go:\n  r.Use(middleware.%s())", data.Name)
		}
	default:
		return nil, "", diagnostic("generator_unknown", "generate", request.Kind, fmt.Errorf("unknown generator %q", request.Kind), "Run gin-kit generate --help for the available generators.")
	}
	return files, nextSteps, nil
}

// migrationPath performs this package operation.
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

// resourceNextSteps performs this package operation.
func resourceNextSteps(m Manifest, data genData, handlerPackage string) string {
	var wiring string
	if m.ProjectType == "runtime" {
		wiring = fmt.Sprintf(`In internal/app/app.go, after the existing Register call, add:

    %s := service.New%sService(repository.New%sRepository(application.Database()))
    %s.Register%s(router, application.OpenAPI(), %s)

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

// generateCommand performs this package operation.
func generateCommand() *cobra.Command {
	var dryRun bool
	generate := &cobra.Command{Use: "generate", Short: "Generate project building blocks from real, manifest-aware templates"}
	generate.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show generated files without writing them")

	runKind := func(kind string, fields, table *string, softDelete *bool) func(*cobra.Command, []string) error {
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
			if softDelete != nil {
				request.SoftDelete = *softDelete
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
	resourceSoftDelete := resource.Flags().Bool("soft-delete", false, "add deleted_at and make Delete a soft delete")
	resource.RunE = runKind("resource", resourceFields, resourceTable, resourceSoftDelete)
	generate.AddCommand(resource)

	domain := &cobra.Command{
		Use:   "domain <Name>",
		Short: "Generate a domain model with its repository interface",
		Args:  cobra.ExactArgs(1),
	}
	domainFields := domain.Flags().String("fields", "", fieldsGrammar)
	domain.RunE = runKind("domain", domainFields, nil, nil)
	generate.AddCommand(domain)

	dtoCommand := &cobra.Command{
		Use:   "dto <Name>",
		Short: "Generate request and response DTOs for an existing domain model",
		Args:  cobra.ExactArgs(1),
	}
	dtoFields := dtoCommand.Flags().String("fields", "", fieldsGrammar+" (must match the domain model)")
	dtoCommand.RunE = runKind("dto", dtoFields, nil, nil)
	generate.AddCommand(dtoCommand)

	factoryCommand := &cobra.Command{
		Use:   "factory <Name>",
		Short: "Generate a model factory with field-aware fake data",
		Args:  cobra.ExactArgs(1),
	}
	factoryFields := factoryCommand.Flags().String("fields", "", fieldsGrammar+" (must match the domain model)")
	factoryCommand.RunE = runKind("factory", factoryFields, nil, nil)
	generate.AddCommand(factoryCommand)

	repository := &cobra.Command{
		Use:   "repository <Name>",
		Short: "Generate a repository implementation for an existing domain model",
		Args:  cobra.ExactArgs(1),
	}
	repositoryFields := repository.Flags().String("fields", "", fieldsGrammar+" (must match the domain model)")
	repository.RunE = runKind("repository", repositoryFields, nil, nil)
	generate.AddCommand(repository)

	for _, kind := range []string{"service", "handler", "middleware", "policy", "seeder", "job", "event", "mail"} {
		k := kind
		generate.AddCommand(&cobra.Command{
			Use:   k + " <Name>",
			Short: "Generate a " + k,
			Args:  cobra.ExactArgs(1),
			RunE:  runKind(k, nil, nil, nil),
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
	var from, out, lang string
	client := &cobra.Command{Use: "client", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if lang != "ts" {
			return errors.New("only --lang ts is supported")
		}
		root, m, err := projectRoot()
		if err != nil {
			return err
		}
		source := from
		var spec *clientSpec
		if source == "" {
			if m.ProjectType == "standalone" {
				source = filepath.Join(root, "api", "openapi.yaml")
			} else {
				p := filepath.Join(root, "cmd", "server", "main.go")
				d, e := os.ReadFile(p)
				if e != nil {
					return e
				}
				if !bytes.Contains(d, []byte("--openapi")) {
					return diagnostic("openapi_flag_missing", "generate client", p, errors.New("server does not support --openapi"), "Add --openapi or pass --from URL|file")
				}
				spec, err = loadRuntimeOpenAPI(root)
			}
		}
		if spec == nil && err == nil {
			spec, err = loadClientSpec(source)
		}
		if err != nil {
			return diagnostic("openapi_load_failed", "generate client", source, err, "Pass a valid OpenAPI file or URL")
		}
		if out == "" {
			out = filepath.Join(root, "web", "client", "api.ts")
		}
		if dryRun {
			fmt.Println(out)
			return nil
		}
		if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
			return err
		}
		return os.WriteFile(out, renderTSClient(spec), 0644)
	}}
	client.Flags().StringVar(&lang, "lang", "ts", "client language")
	client.Flags().StringVar(&from, "from", "", "OpenAPI URL or file")
	client.Flags().StringVar(&out, "out", "", "output file")
	generate.AddCommand(client)
	return generate
}
