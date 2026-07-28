package openapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// nested defines an implementation type used by this package.
type nested struct {
	// Label store data used by this type.
	Label string `json:"label"`
}

// node defines an implementation type used by this package.
type node struct {
	// Value store data used by this type.
	Value string `json:"value"`
	// Next store data used by this type.
	Next *node `json:"next"`
}

// sample defines an implementation type used by this package.
type sample struct {
	// Title store data used by this type.
	Title string `json:"title" validate:"required,min=3,max=255"`
	// Count store data used by this type.
	Count int64 `json:"count"`
	// Price store data used by this type.
	Price float64 `json:"price"`
	// Done store data used by this type.
	Done bool `json:"done"`
	// Due store data used by this type.
	Due time.Time `json:"due"`
	// Notes store data used by this type.
	Notes *string `json:"notes"`
	// Tags store data used by this type.
	Tags []string `json:"tags"`
	// Labels store data used by this type.
	Labels map[string]string `json:"labels"`
	// Nested store data used by this type.
	Nested nested `json:"nested"`
	// Secret store data used by this type.
	Secret string `json:"-"`
	// Stringly store data used by this type.
	Stringly int `json:"stringly,string"`
	// unexposed store data used by this type.
	unexposed string
}

func TestSchemaForCoversTypes(t *testing.T) {
	defs := map[string]*Schema{}
	schema := schemaFor(sample{}, defs)
	if schema.Ref != "#/components/schemas/Sample" {
		t.Fatalf("named struct not registered: %+v", schema)
	}
	component := defs["Sample"]
	properties := component.Properties
	if properties["title"].Type != "string" || *properties["title"].MinLength != 3 || *properties["title"].MaxLength != 255 {
		t.Fatalf("title constraints wrong: %+v", properties["title"])
	}
	if len(component.Required) != 1 || component.Required[0] != "title" {
		t.Fatalf("required wrong: %v", component.Required)
	}
	if properties["count"].Format != "int64" || properties["price"].Format != "double" || properties["done"].Type != "boolean" {
		t.Fatalf("scalars wrong: %+v", properties)
	}
	if properties["due"].Format != "date-time" {
		t.Fatalf("time wrong: %+v", properties["due"])
	}
	if !properties["notes"].Nullable || properties["notes"].Type != "string" {
		t.Fatalf("pointer wrong: %+v", properties["notes"])
	}
	if properties["tags"].Type != "array" || properties["tags"].Items.Type != "string" {
		t.Fatalf("slice wrong: %+v", properties["tags"])
	}
	if properties["labels"].AdditionalProperties == nil {
		t.Fatalf("map wrong: %+v", properties["labels"])
	}
	if properties["nested"].Ref == "" || defs["Nested"] == nil {
		t.Fatalf("nested component missing: %+v", properties["nested"])
	}
	if _, leaked := properties["Secret"]; leaked {
		t.Fatal("json:\"-\" field leaked")
	}
	if properties["stringly"].Type != "string" {
		t.Fatalf(",string option ignored: %+v", properties["stringly"])
	}
	if _, leaked := properties["unexposed"]; leaked {
		t.Fatal("unexported field leaked")
	}
}

func TestSchemaForTerminatesOnCycles(t *testing.T) {
	defs := map[string]*Schema{}
	schemaFor(node{}, defs)
	next := defs["Node"].Properties["next"]
	if len(next.AllOf) != 1 || next.AllOf[0].Ref != "#/components/schemas/Node" {
		t.Fatalf("cycle not handled via ref: %+v", next)
	}
}

// buildDocument performs this package operation.
func buildDocument(t *testing.T, registry *Registry, routes []Route) *Document {
	t.Helper()
	return registry.Build(BuildOptions{
		Info:   Info{Title: "Test API", Version: "1.0.0"},
		Routes: routes,
	})
}

func TestBuildDocumentsUnregisteredRoutes(t *testing.T) {
	document := buildDocument(t, NewRegistry(), []Route{
		{Method: "GET", Path: "/api/v1/tickets/:id"},
		{Method: "GET", Path: "/health/live"},
	})
	item := document.Paths["/api/v1/tickets/{id}"]
	if item == nil || item.Get == nil {
		t.Fatalf("default op missing: %+v", document.Paths)
	}
	if item.Get.Tags[0] != "tickets" {
		t.Fatalf("tag derivation wrong: %v", item.Get.Tags)
	}
	if len(item.Get.Parameters) != 1 || item.Get.Parameters[0].Name != "id" || item.Get.Parameters[0].In != "path" {
		t.Fatalf("path param missing: %+v", item.Get.Parameters)
	}
	if document.Paths["/health/live"].Get.Tags[0] != "health" {
		t.Fatalf("health tag wrong")
	}
}

func TestBuildEnrichesDescribedOperations(t *testing.T) {
	registry := NewRegistry()
	registry.Describe(
		Operation{
			Method: "POST", Path: "/api/v1/tickets",
			Summary: "Create", Request: sample{}, Response: sample{}, Status: 201,
		},
		Operation{
			Method: "GET", Path: "/api/v1/tickets",
			Response: sample{}, List: true,
			Filters: []string{"done", "title"}, Sorts: []string{"created_at"},
		},
		Operation{
			Method: "DELETE", Path: "/api/v1/tickets/:id",
			Status: 204, ErrorCodes: []string{"ticket_not_found"}, Security: true,
		},
		Operation{Method: "GET", Path: "/internal/hidden", Hidden: true},
	)
	document := buildDocument(t, registry, []Route{
		{Method: "POST", Path: "/api/v1/tickets"},
		{Method: "GET", Path: "/api/v1/tickets"},
		{Method: "DELETE", Path: "/api/v1/tickets/:id"},
		{Method: "GET", Path: "/internal/hidden"},
	})

	create := document.Paths["/api/v1/tickets"].Post
	if create.RequestBody == nil || create.Responses["201"] == nil || create.Responses["422"] == nil {
		t.Fatalf("create op incomplete: %+v", create.Responses)
	}

	list := document.Paths["/api/v1/tickets"].Get
	var names []string
	for _, param := range list.Parameters {
		names = append(names, param.Name)
	}
	joined := strings.Join(names, ",")
	for _, expected := range []string{"filter[done]", "filter[title]", "sort", "page", "per_page"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("list params missing %s: %v", expected, names)
		}
	}
	listSchema := list.Responses["200"].Content["application/json"].Schema
	if listSchema.Properties["data"].Type != "array" || listSchema.Properties["meta"].Ref == "" {
		t.Fatalf("list envelope wrong: %+v", listSchema)
	}

	remove := document.Paths["/api/v1/tickets/{id}"].Delete
	if remove.Responses["204"] == nil || remove.Responses["404"] == nil || remove.Responses["401"] == nil {
		t.Fatalf("delete responses wrong: %v", remove.Responses)
	}
	if len(remove.Security) != 1 {
		t.Fatalf("security requirement missing: %+v", remove.Security)
	}
	if document.Components.SecuritySchemes["bearerAuth"] == nil {
		t.Fatal("bearer scheme missing")
	}

	if _, present := document.Paths["/internal/hidden"]; present {
		t.Fatal("hidden route documented")
	}
}

func TestBuildExcludesInfrastructurePaths(t *testing.T) {
	registry := NewRegistry()
	document := registry.Build(BuildOptions{
		Info: Info{Title: "T", Version: "1"},
		Routes: []Route{
			{Method: "GET", Path: "/docs"},
			{Method: "GET", Path: "/openapi.json"},
			{Method: "GET", Path: "/metrics"},
			{Method: "GET", Path: "/debug/pprof/heap"},
			{Method: "GET", Path: "/kept"},
		},
		ExcludePaths:    []string{"/docs", "/openapi.json", "/metrics"},
		ExcludePrefixes: []string{"/debug/pprof"},
	})
	if len(document.Paths) != 1 || document.Paths["/kept"] == nil {
		t.Fatalf("exclusions wrong: %v", document.Paths)
	}
}

func TestDocumentMarshalsToValidJSON(t *testing.T) {
	registry := NewRegistry()
	registry.Describe(Operation{Method: "GET", Path: "/things", Response: sample{}, List: true})
	document := buildDocument(t, registry, []Route{{Method: "GET", Path: "/things"}})
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var round map[string]any
	if err := json.Unmarshal(payload, &round); err != nil {
		t.Fatal(err)
	}
	if round["openapi"] != "3.0.3" {
		t.Fatalf("version wrong: %v", round["openapi"])
	}
}
