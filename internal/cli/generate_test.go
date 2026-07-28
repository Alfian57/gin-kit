package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// generatedProject performs this package operation.
func generatedProject(t *testing.T, m Manifest) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), m.Project)
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fileContent performs this package operation.
func fileContent(t *testing.T, files map[string][]byte, suffix string) string {
	t.Helper()
	for path, content := range files {
		if strings.HasSuffix(filepath.ToSlash(path), suffix) {
			return string(content)
		}
	}
	keys := make([]string, 0, len(files))
	for path := range files {
		keys = append(keys, path)
	}
	t.Fatalf("no generated file ends with %s; got %v", suffix, keys)
	return ""
}

func TestRunGenerateResourceStandaloneAPI(t *testing.T) {
	m := Manifest{
		Version: 3, ProjectType: "standalone", Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "postgres", ORM: "sqlx",
	}
	rootDir := generatedProject(t, m)
	files, nextSteps, err := runGenerate(rootDir, m, generateRequest{
		Kind: "resource", Name: "Ticket",
		Fields: "title:string,done:bool,price:float64,due_at:time",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 9 {
		t.Fatalf("expected 9 files, got %d", len(files))
	}

	dtoFile := fileContent(t, files, "internal/dto/ticket_dto.go")
	for _, want := range []string{
		"type CreateTicketRequest struct", "type UpdateTicketRequest struct",
		"type TicketResponse struct", "func NewTicketResponse(entity domain.Ticket) TicketResponse",
		"func NewTicketResponseList(entities []domain.Ticket) []TicketResponse",
		"func (r *CreateTicketRequest) Normalize()",
		"CreatedAt time.Time `json:\"created_at\"`",
	} {
		if !strings.Contains(dtoFile, want) {
			t.Errorf("dto missing %q:\n%s", want, dtoFile)
		}
	}
	if strings.Contains(dtoFile, "db:") || strings.Contains(dtoFile, "gorm:") {
		t.Errorf("dto carries persistence tags:\n%s", dtoFile)
	}

	factoryFile := fileContent(t, files, "internal/database/factories/ticket_factory.go")
	for _, want := range []string{
		"factory.Define(func(f *factory.F) domain.Ticket",
		"example.com/shop/internal/platform/factory",
		"f.Bool()", "f.Price(1, 1000)", "f.PastDate().UTC()",
	} {
		if !strings.Contains(factoryFile, want) {
			t.Errorf("factory missing %q:\n%s", want, factoryFile)
		}
	}

	domain := fileContent(t, files, "internal/domain/ticket.go")
	for _, want := range []string{
		"type Ticket struct", `func (Ticket) TableName() string { return "tickets" }`,
		"TicketRepository interface", "example.com/shop/internal/platform/query",
		"DueAt time.Time",
	} {
		if !strings.Contains(domain, want) {
			t.Errorf("domain missing %q:\n%s", want, domain)
		}
	}

	repository := fileContent(t, files, "internal/repository/ticket_repository.go")
	for _, want := range []string{
		"SelectContext", "BuildCountSQL", "Rebind",
		"INSERT INTO tickets (id, title, done, price, due_at, created_at, updated_at)",
		"example.com/shop/internal/platform/database",
	} {
		if !strings.Contains(repository, want) {
			t.Errorf("sqlx repository missing %q", want)
		}
	}
	if strings.Contains(repository, "gorm") {
		t.Error("sqlx repository mentions gorm")
	}

	handler := fileContent(t, files, "internal/handler/api/ticket_handler.go")
	for _, want := range []string{
		"package api", "RegisterTickets(group *gin.RouterGroup",
		`query.Partial("title"), query.Exact("done").Bool()`,
		"httpx.BindJSON[dto.CreateTicketRequest]", "httpx.BindJSON[dto.UpdateTicketRequest]",
		"dto.NewTicketResponse(created)", "dto.NewTicketResponseList(items)", "httpx.Fail",
	} {
		if !strings.Contains(handler, want) {
			t.Errorf("handler missing %q:\n%s", want, handler)
		}
	}
	if strings.Contains(handler, "platform/response") {
		t.Error("standalone handler still imports legacy platform/response")
	}
	handlerTest := fileContent(t, files, "internal/handler/api/ticket_handler_test.go")
	for _, want := range []string{"http.StatusUnprocessableEntity", `"fields"`} {
		if !strings.Contains(handlerTest, want) {
			t.Errorf("handler test missing %q:\n%s", want, handlerTest)
		}
	}

	migration := fileContent(t, files, ".sql")
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS tickets", "price DOUBLE PRECISION NOT NULL",
		"done BOOLEAN NOT NULL DEFAULT FALSE", "DROP TABLE IF EXISTS tickets;",
	} {
		if !strings.Contains(migration, want) {
			t.Errorf("migration missing %q:\n%s", want, migration)
		}
	}

	if !strings.Contains(nextSteps, "api.RegisterTickets(r, service.NewTicketService(repository.NewTicketRepository(db)))") {
		t.Errorf("next steps wiring snippet wrong:\n%s", nextSteps)
	}
}

func TestRunGenerateResourceRuntimeGORM(t *testing.T) {
	m := Manifest{
		Version: 3, ProjectType: "runtime", RuntimeVersion: "0.3.0",
		Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, m)
	files, nextSteps, err := runGenerate(rootDir, m, generateRequest{Kind: "resource", Name: "Ticket", Fields: "title:string"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 10 {
		t.Fatalf("expected 10 files, got %d", len(files))
	}
	repository := fileContent(t, files, "internal/repository/ticket_repository.go")
	for _, want := range []string{
		"github.com/Alfian57/gin-kit/runtime/database", "ApplyGORM", "CountGORM", "gorm.ErrRecordNotFound",
	} {
		if !strings.Contains(repository, want) {
			t.Errorf("gorm repository missing %q", want)
		}
	}
	handler := fileContent(t, files, "internal/handler/api/ticket_handler.go")
	for _, want := range []string{
		"httpx.BindJSON[dto.CreateTicketRequest]",
		"Request: dto.CreateTicketRequest{}, Response: dto.TicketResponse{}",
		"github.com/Alfian57/gin-kit/runtime/query",
		"httpx.List(c, dto.NewTicketResponseList(items), q.Meta(total))",
	} {
		if !strings.Contains(handler, want) {
			t.Errorf("runtime handler missing %q:\n%s", want, handler)
		}
	}
	serviceFile := fileContent(t, files, "internal/service/ticket_service.go")
	if !strings.Contains(serviceFile, "Create(ctx context.Context, request dto.CreateTicketRequest)") ||
		strings.Contains(serviceFile, "ErrInvalidTicket") {
		t.Errorf("service must take DTOs and drop sentinel validation:\n%s", serviceFile)
	}
	if !strings.Contains(nextSteps, "application.Database()") {
		t.Errorf("runtime wiring snippet wrong:\n%s", nextSteps)
	}
}

func TestRunGenerateResourceSoftDelete(t *testing.T) {
	manifests := map[string]Manifest{
		"sqlx": {
			Version: 3, ProjectType: "standalone", Project: "shop", Module: "example.com/shop",
			Mode: "api", Database: "postgres", ORM: "sqlx",
		},
		"gorm": {
			Version: 3, ProjectType: "runtime", RuntimeVersion: "0.3.0",
			Project: "shop", Module: "example.com/shop",
			Mode: "api", Database: "sqlite", ORM: "gorm",
		},
	}
	for name, m := range manifests {
		t.Run(name, func(t *testing.T) {
			rootDir := generatedProject(t, m)
			files, _, err := runGenerate(rootDir, m, generateRequest{
				Kind: "resource", Name: "Archive", Fields: "title:string", SoftDelete: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			expected := 9
			if m.ProjectType == "runtime" {
				expected = 10
			}
			if len(files) != expected {
				t.Fatalf("expected %d files, got %d", expected, len(files))
			}

			domainFile := fileContent(t, files, "internal/domain/archive.go")
			if !strings.Contains(domainFile, "DeletedAt *time.Time `json:\"-\" db:\"deleted_at\" gorm:\"index\"`") {
				t.Errorf("domain missing soft-delete field:\n%s", domainFile)
			}

			repository := fileContent(t, files, "internal/repository/archive_repository.go")
			if !strings.Contains(repository, "deleted_at IS NULL") {
				t.Errorf("repository missing deleted_at filter:\n%s", repository)
			}
			if m.ORM == "sqlx" {
				if strings.Contains(repository, "DELETE FROM") {
					t.Errorf("sqlx soft delete still hard-deletes:\n%s", repository)
				}
				for _, want := range []string{
					"UPDATE archives SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL",
					`where, whereArgs := q.WhereSQL()`,
					`conditions := "deleted_at IS NULL"`,
					"LIMIT ? OFFSET ?",
				} {
					if !strings.Contains(repository, want) {
						t.Errorf("sqlx soft-delete repository missing %q:\n%s", want, repository)
					}
				}
				if strings.Contains(repository, "BuildSQL") || strings.Contains(repository, "BuildCountSQL") {
					t.Errorf("sqlx soft-delete List must compose SQL manually:\n%s", repository)
				}
			} else {
				if !strings.Contains(repository, `Update("deleted_at", time.Now().UTC())`) {
					t.Errorf("gorm soft delete must update deleted_at:\n%s", repository)
				}
			}

			migration := fileContent(t, files, ".sql")
			for _, want := range []string{
				"deleted_at TIMESTAMP NULL DEFAULT NULL",
				"CREATE INDEX archives_deleted_at_idx ON archives (deleted_at);",
			} {
				if !strings.Contains(migration, want) {
					t.Errorf("migration missing %q:\n%s", want, migration)
				}
			}

			// Transport and factory shapes stay untouched: deleted_at is
			// repository-internal state, never bound or faked.
			for _, suffix := range []string{"internal/dto/archive_dto.go", "internal/database/factories/archive_factory.go"} {
				content := fileContent(t, files, suffix)
				if strings.Contains(content, "deleted_at") || strings.Contains(content, "DeletedAt") {
					t.Errorf("%s mentions deleted_at:\n%s", suffix, content)
				}
			}

			if m.ProjectType == "runtime" {
				repositoryTest := fileContent(t, files, "internal/repository/archive_repository_test.go")
				for _, want := range []string{
					"soft delete must keep the row",
					"soft-deleted row still listed",
					"deleted_at TIMESTAMP)", // SQLiteSchema carries the column
				} {
					if !strings.Contains(repositoryTest, want) {
						t.Errorf("repository test missing %q:\n%s", want, repositoryTest)
					}
				}
			}
		})
	}

	// Regression: without the flag, no generated file mentions deleted_at.
	m := manifests["gorm"]
	rootDir := generatedProject(t, m)
	files, _, err := runGenerate(rootDir, m, generateRequest{Kind: "resource", Name: "Archive", Fields: "title:string"})
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		if strings.Contains(string(content), "deleted_at") || strings.Contains(string(content), "DeletedAt") {
			t.Errorf("%s mentions deleted_at without --soft-delete", path)
		}
	}
}

func TestRunGenerateResourceStandaloneUI(t *testing.T) {
	m := Manifest{
		Version: 3, ProjectType: "standalone", Project: "portal", Module: "example.com/portal",
		Mode: "ui", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, m)
	files, _, err := runGenerate(rootDir, m, generateRequest{Kind: "resource", Name: "Ticket", Fields: "title:string"})
	if err != nil {
		t.Fatal(err)
	}
	handler := fileContent(t, files, "internal/handler/web/ticket_handler.go")
	if !strings.Contains(handler, "session.TemplateField(c)") || !strings.Contains(handler, `"tickets.html"`) {
		t.Errorf("web handler missing session/template wiring:\n%s", handler)
	}
	page := fileContent(t, files, "web/templates/tickets.html")
	if !strings.Contains(page, "{{range .items}}") || !strings.Contains(page, "{{.csrf_field}}") ||
		!strings.Contains(page, `name="title"`) {
		t.Errorf("web template runtime actions wrong:\n%s", page)
	}
}

func TestRunGenerateSinglesAndCollisions(t *testing.T) {
	m := Manifest{
		Version: 3, ProjectType: "standalone", Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, m)

	files, _, err := runGenerate(rootDir, m, generateRequest{Kind: "middleware", Name: "RequestTimer"})
	if err != nil {
		t.Fatal(err)
	}
	middleware := fileContent(t, files, "internal/middleware/request_timer.go")
	if !strings.Contains(middleware, "func RequestTimer() gin.HandlerFunc") {
		t.Errorf("middleware content wrong:\n%s", middleware)
	}

	if _, _, err := runGenerate(rootDir, m, generateRequest{Kind: "resource", Name: "type"}); err == nil {
		t.Fatal("keyword name accepted")
	}
	if _, _, err := runGenerate(rootDir, m, generateRequest{Kind: "resource", Name: "Ticket", Fields: "id:string"}); err == nil {
		t.Fatal("reserved field accepted")
	}

	// Collision preflight: publish once, second run against same files fails
	// without writing anything.
	files, _, err = runGenerate(rootDir, m, generateRequest{Kind: "domain", Name: "Invoice"})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeGeneratedFiles(rootDir, files, false); err != nil {
		t.Fatal(err)
	}
	files, _, err = runGenerate(rootDir, m, generateRequest{Kind: "domain", Name: "Invoice"})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeGeneratedFiles(rootDir, files, false); err == nil {
		t.Fatal("collision not detected")
	}
}

func TestRunGenerateFactoryKind(t *testing.T) {
	runtime := Manifest{
		Version: 3, ProjectType: "runtime", RuntimeVersion: "0.3.0",
		Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, runtime)
	files, nextSteps, err := runGenerate(rootDir, runtime, generateRequest{
		Kind: "factory", Name: "Profile", Fields: "email:string,age:int,bio:text?",
	})
	if err != nil {
		t.Fatal(err)
	}
	content := fileContent(t, files, "internal/database/factories/profile_factory.go")
	for _, want := range []string{
		"github.com/Alfian57/gin-kit/runtime/factory",
		"f.Email()", "f.Number(1, 1000)",
		"ptr(", "func ptr[T any](value T) *T",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("factory missing %q:\n%s", want, content)
		}
	}
	if !strings.Contains(nextSteps, "NewProfileFactory().Create(ctx, repo.Create)") {
		t.Errorf("factory next steps wrong:\n%s", nextSteps)
	}

	// Foreign-key-shaped string fields fake as UUIDs, not word strings.
	files, _, err = runGenerate(rootDir, runtime, generateRequest{
		Kind: "factory", Name: "Membership", Fields: "user_id:string,email:string",
	})
	if err != nil {
		t.Fatal(err)
	}
	content = fileContent(t, files, "internal/database/factories/membership_factory.go")
	if !strings.Contains(content, "UserId: f.UUID()") || !strings.Contains(content, "Email: f.Email()") {
		t.Errorf("foreign-key fake shape wrong:\n%s", content)
	}
}

func TestRunGenerateDTOKind(t *testing.T) {
	m := Manifest{
		Version: 3, ProjectType: "standalone", Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, m)
	files, nextSteps, err := runGenerate(rootDir, m, generateRequest{
		Kind: "dto", Name: "Profile", Fields: "email:string,nickname:string?,password:string",
	})
	if err != nil {
		t.Fatal(err)
	}
	content := fileContent(t, files, "internal/dto/profile_dto.go")
	for _, want := range []string{
		"type CreateProfileRequest struct", "type ProfileResponse struct",
		`validate:"required,max=255"`, `Nickname *string ` + "`" + `json:"nickname" validate:"omitempty,max=255"` + "`",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("dto missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(nextSteps, "generate domain Profile") == false {
		t.Errorf("dto next steps wrong:\n%s", nextSteps)
	}

	// The response type must exclude the credential-like field entirely.
	responseSection := content[strings.Index(content, "type ProfileResponse struct"):]
	if strings.Contains(responseSection, "Password") {
		t.Errorf("sensitive field leaked into response:\n%s", responseSection)
	}
	if !strings.Contains(content, "Password string") {
		t.Error("sensitive field missing from create request")
	}
}

func TestRunGenerateRuntimeOnlyKinds(t *testing.T) {
	runtime := Manifest{
		Version: 3, ProjectType: "runtime", RuntimeVersion: "0.3.0",
		Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, runtime)

	files, nextSteps, err := runGenerate(rootDir, runtime, generateRequest{Kind: "job", Name: "SendInvoice"})
	if err != nil {
		t.Fatal(err)
	}
	job := fileContent(t, files, "internal/jobs/send_invoice.go")
	if !strings.Contains(job, `const TypeSendInvoice = "send_invoice"`) ||
		!strings.Contains(job, "func HandleSendInvoice(ctx context.Context, payload SendInvoicePayload) error") {
		t.Errorf("job content wrong:\n%s", job)
	}
	if !strings.Contains(nextSteps, "queue.Register(q, TypeSendInvoice, HandleSendInvoice)") {
		t.Errorf("job next steps wrong:\n%s", nextSteps)
	}

	files, _, err = runGenerate(rootDir, runtime, generateRequest{Kind: "event", Name: "OrderPlaced"})
	if err != nil {
		t.Fatal(err)
	}
	eventContent := fileContent(t, files, "internal/event/order_placed.go")
	if !strings.Contains(eventContent, "type OrderPlaced struct") ||
		!strings.Contains(eventContent, "func OnOrderPlaced(ctx context.Context, payload OrderPlaced) error") {
		t.Errorf("event content wrong:\n%s", eventContent)
	}

	files, _, err = runGenerate(rootDir, runtime, generateRequest{Kind: "mail", Name: "Welcome"})
	if err != nil {
		t.Fatal(err)
	}
	mailContent := fileContent(t, files, "internal/mail/welcome.go")
	if !strings.Contains(mailContent, "runtimemail.NewMessage()") ||
		!strings.Contains(mailContent, `HTMLTemplate(templates, "welcome.html", nil)`) {
		t.Errorf("mail content wrong:\n%s", mailContent)
	}
	page := fileContent(t, files, "web/templates/mail/welcome.html")
	if !strings.Contains(page, "<h1>Welcome</h1>") {
		t.Errorf("mail template wrong:\n%s", page)
	}

	standalone := Manifest{
		Version: 3, ProjectType: "standalone", Project: "plain", Module: "example.com/plain",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	standaloneDir := generatedProject(t, standalone)
	for _, kind := range []string{"job", "event", "mail"} {
		if _, _, err := runGenerate(standaloneDir, standalone, generateRequest{Kind: kind, Name: "Thing"}); err == nil {
			t.Errorf("%s generator accepted for standalone project type", kind)
		}
	}
}

func TestRunGeneratePolicyKind(t *testing.T) {
	runtime := Manifest{
		Version: 3, ProjectType: "runtime", RuntimeVersion: "0.3.0",
		Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, runtime)
	files, nextSteps, err := runGenerate(rootDir, runtime, generateRequest{Kind: "policy", Name: "Ticket"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	content := fileContent(t, files, "internal/policy/ticket_policy.go")
	for _, want := range []string{
		"github.com/Alfian57/gin-kit/runtime/authz",
		"authz.Decision", `authz.Deny("unauthenticated")`, "authz.Allow()",
		"func (p *TicketPolicy) CanView(ctx context.Context, subjectID string, resource domain.Ticket) authz.Decision",
		"func (p *TicketPolicy) CanCreate(ctx context.Context, subjectID string) authz.Decision",
		"func (p *TicketPolicy) CanUpdate(ctx context.Context, subjectID string, resource domain.Ticket) authz.Decision",
		"func (p *TicketPolicy) CanDelete(ctx context.Context, subjectID string, resource domain.Ticket) authz.Decision",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("policy missing %q:\n%s", want, content)
		}
	}
	if !strings.Contains(nextSteps, "auth.UserID(c)") || !strings.Contains(nextSteps, "generate domain Ticket") {
		t.Errorf("runtime policy next steps wrong:\n%s", nextSteps)
	}

	// A scaffolded standalone project already vendors internal/platform/authz,
	// so the generator emits only the policy pair.
	standalone := Manifest{
		Version: 3, ProjectType: "standalone", Project: "plain", Module: "example.com/plain",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	standaloneDir := generatedProject(t, standalone)
	files, nextSteps, err = runGenerate(standaloneDir, standalone, generateRequest{Kind: "policy", Name: "Ticket"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files on a scaffolded standalone, got %d", len(files))
	}
	content = fileContent(t, files, "internal/policy/ticket_policy.go")
	if !strings.Contains(content, "example.com/plain/internal/platform/authz") {
		t.Errorf("standalone policy missing vendored authz import:\n%s", content)
	}
	if !strings.Contains(nextSteps, `c.GetString("user_id")`) {
		t.Errorf("standalone policy next steps wrong:\n%s", nextSteps)
	}

	// A standalone project missing the vendored package gets it back-filled.
	bareDir := t.TempDir()
	files, _, err = runGenerate(bareDir, standalone, generateRequest{Kind: "policy", Name: "Ticket"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files with the authz back-fill, got %d", len(files))
	}
	platform := fileContent(t, files, "internal/platform/authz/authz.go")
	for _, want := range []string{
		"package authz", "example.com/plain/internal/platform/httpx",
		`"forbidden"`, "You are not allowed to perform this action.",
	} {
		if !strings.Contains(platform, want) {
			t.Errorf("back-filled authz missing %q:\n%s", want, platform)
		}
	}
	testFile := fileContent(t, files, "internal/policy/ticket_policy_test.go")
	for _, want := range []string{"CanView", "CanCreate", "CanUpdate", "CanDelete", "example.com/plain/internal/domain"} {
		if !strings.Contains(testFile, want) {
			t.Errorf("policy test missing %q:\n%s", want, testFile)
		}
	}
}

func TestWriteGeneratedFilesFormatsStagedGo(t *testing.T) {
	rootDir := t.TempDir()
	unformatted := []byte("package demo\n\nfunc   Weird(  ) {}\n")
	target := filepath.Join(rootDir, "internal", "demo", "weird.go")
	if err := writeGeneratedFiles(rootDir, map[string][]byte{target: unformatted}, false); err != nil {
		t.Fatal(err)
	}
	published, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(published), "func   Weird") {
		t.Fatalf("published file was not gofmt-ed:\n%s", published)
	}
}
