package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generatedProject(t *testing.T, m Manifest) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), m.Project)
	if err := scaffold(dir, m); err != nil {
		t.Fatal(err)
	}
	return dir
}

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

func TestRunGenerateResourceStarterAPI(t *testing.T) {
	m := Manifest{
		Version: 2, Edition: "starter", Project: "shop", Module: "example.com/shop",
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
	if len(files) != 8 {
		t.Fatalf("expected 8 files, got %d", len(files))
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
		`validate:"required,max=255"`, "httpx.BindJSON[ticketInput]", "httpx.Fail",
	} {
		if !strings.Contains(handler, want) {
			t.Errorf("handler missing %q:\n%s", want, handler)
		}
	}
	if strings.Contains(handler, "platform/response") {
		t.Error("starter handler still imports legacy platform/response")
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

func TestRunGenerateResourceFrameworkGORM(t *testing.T) {
	m := Manifest{
		Version: 2, Edition: "framework", FrameworkVersion: "0.3.0",
		Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, m)
	files, nextSteps, err := runGenerate(rootDir, m, generateRequest{Kind: "resource", Name: "Ticket", Fields: "title:string"})
	if err != nil {
		t.Fatal(err)
	}
	repository := fileContent(t, files, "internal/repository/ticket_repository.go")
	for _, want := range []string{
		"github.com/Alfian57/gin-kit/framework/database", "ApplyGORM", "CountGORM", "gorm.ErrRecordNotFound",
	} {
		if !strings.Contains(repository, want) {
			t.Errorf("gorm repository missing %q", want)
		}
	}
	handler := fileContent(t, files, "internal/handler/api/ticket_handler.go")
	for _, want := range []string{
		"httpx.BindJSON[ticketInput]", `validate:"required,max=255"`,
		"github.com/Alfian57/gin-kit/framework/query", "httpx.List(c, items, q.Meta(total))",
	} {
		if !strings.Contains(handler, want) {
			t.Errorf("framework handler missing %q:\n%s", want, handler)
		}
	}
	if !strings.Contains(nextSteps, "application.Database()") {
		t.Errorf("framework wiring snippet wrong:\n%s", nextSteps)
	}
}

func TestRunGenerateResourceStarterUI(t *testing.T) {
	m := Manifest{
		Version: 2, Edition: "starter", Project: "portal", Module: "example.com/portal",
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
		Version: 2, Edition: "starter", Project: "shop", Module: "example.com/shop",
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
	framework := Manifest{
		Version: 2, Edition: "framework", FrameworkVersion: "0.3.0",
		Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, framework)
	files, nextSteps, err := runGenerate(rootDir, framework, generateRequest{
		Kind: "factory", Name: "Profile", Fields: "email:string,age:int,bio:text?",
	})
	if err != nil {
		t.Fatal(err)
	}
	content := fileContent(t, files, "internal/database/factories/profile_factory.go")
	for _, want := range []string{
		"github.com/Alfian57/gin-kit/framework/factory",
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
}

func TestRunGenerateFrameworkOnlyKinds(t *testing.T) {
	framework := Manifest{
		Version: 2, Edition: "framework", FrameworkVersion: "0.3.0",
		Project: "shop", Module: "example.com/shop",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	rootDir := generatedProject(t, framework)

	files, nextSteps, err := runGenerate(rootDir, framework, generateRequest{Kind: "job", Name: "SendInvoice"})
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

	files, _, err = runGenerate(rootDir, framework, generateRequest{Kind: "event", Name: "OrderPlaced"})
	if err != nil {
		t.Fatal(err)
	}
	eventContent := fileContent(t, files, "internal/event/order_placed.go")
	if !strings.Contains(eventContent, "type OrderPlaced struct") ||
		!strings.Contains(eventContent, "func OnOrderPlaced(ctx context.Context, payload OrderPlaced) error") {
		t.Errorf("event content wrong:\n%s", eventContent)
	}

	files, _, err = runGenerate(rootDir, framework, generateRequest{Kind: "mail", Name: "Welcome"})
	if err != nil {
		t.Fatal(err)
	}
	mailContent := fileContent(t, files, "internal/mail/welcome.go")
	if !strings.Contains(mailContent, "frameworkmail.NewMessage()") ||
		!strings.Contains(mailContent, `HTMLTemplate(templates, "welcome.html", nil)`) {
		t.Errorf("mail content wrong:\n%s", mailContent)
	}
	page := fileContent(t, files, "web/templates/mail/welcome.html")
	if !strings.Contains(page, "<h1>Welcome</h1>") {
		t.Errorf("mail template wrong:\n%s", page)
	}

	starter := Manifest{
		Version: 2, Edition: "starter", Project: "plain", Module: "example.com/plain",
		Mode: "api", Database: "sqlite", ORM: "gorm",
	}
	starterDir := generatedProject(t, starter)
	for _, kind := range []string{"job", "event", "mail"} {
		if _, _, err := runGenerate(starterDir, starter, generateRequest{Kind: kind, Name: "Thing"}); err == nil {
			t.Errorf("%s generator accepted for starter edition", kind)
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
