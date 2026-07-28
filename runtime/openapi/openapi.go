// Package openapi builds OpenAPI 3.0.3 documents for gin-kit applications
// without annotations: every live route is documented from the router table,
// and operations described by generated code are enriched with typed
// schemas. Developers never write doc comments — generators emit the
// Describe calls.
package openapi

import (
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Operation describes one HTTP operation. Paths use gin syntax
// ("/api/v1/tickets/:id").
type Operation struct {
	// Method store data used by this type.
	Method string
	// Path store data used by this type.
	Path string
	// Summary store data used by this type.
	Summary string
	// Description store data used by this type.
	Description string
	// Tags store data used by this type.
	Tags []string
	// Request is an instance of the JSON body struct; nil means no body.
	Request any
	// Query is a struct whose `form` fields become query parameters.
	Query any
	// Response is an instance of the success data payload, wrapped as
	// {"data": ...}. nil with Status 204 means no content.
	Response any
	// List wraps Response as {"data": [...], "meta": {...}} and implies
	// pagination parameters.
	List bool
	// Status is the success status code; 0 means 200.
	Status int
	// Filters documents filter[name] query parameters.
	Filters []string
	// Sorts documents the sort parameter's allowed fields.
	Sorts []string
	// Paginated documents page and per_page parameters.
	Paginated bool
	// ErrorCodes lists canonical error codes; statuses are derived.
	ErrorCodes []string
	// Security requires the bearer scheme and documents 401 responses.
	Security bool
	// Hidden excludes the matching route from the document entirely.
	Hidden bool
}

// Registry collects described operations before the document is built.
type Registry struct {
	// mu store data used by this type.
	mu sync.Mutex
	// operations store data used by this type.
	operations []Operation
	// schemeName store data used by this type.
	schemeName string
	// scheme store data used by this type.
	scheme SecurityScheme
}

// NewRegistry performs this package operation.
func NewRegistry() *Registry {
	return &Registry{
		schemeName: "bearerAuth",
		scheme: SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "JWT access token issued by the authentication endpoints.",
		},
	}
}

// Describe registers operations; generated code calls this once per handler
// file.
func (r *Registry) Describe(operations ...Operation) {
	r.mu.Lock()
	r.operations = append(r.operations, operations...)
	r.mu.Unlock()
}

// SetSecurityScheme replaces the default bearer scheme, e.g. with an API key
// scheme.
func (r *Registry) SetSecurityScheme(name string, scheme SecurityScheme) {
	r.mu.Lock()
	r.schemeName = name
	r.scheme = scheme
	r.mu.Unlock()
}

// Route is one live router entry.
type Route struct {
	// Method store data used by this type.
	Method string
	// Path store data used by this type.
	Path string
}

// BuildOptions defines an implementation type used by this package.
type BuildOptions struct {
	// Info store data used by this type.
	Info Info
	// Servers store data used by this type.
	Servers []string
	// Routes store data used by this type.
	Routes []Route
	// ExcludePaths store data used by this type.
	ExcludePaths []string
	// ExcludePrefixes store data used by this type.
	ExcludePrefixes []string
}

// errorStatus maps canonical gin-kit error codes to HTTP statuses.
func errorStatus(code string) string {
	switch code {
	case "not_found", "task_not_found":
		return "404"
	case "method_not_allowed":
		return "405"
	case "invalid_json", "invalid_query", "invalid_path":
		return "400"
	case "body_too_large":
		return "413"
	case "validation_failed":
		return "422"
	case "not_ready":
		return "503"
	case "invalid_token", "missing_token", "invalid_credentials", "invalid_refresh_token":
		return "401"
	case "email_taken":
		return "409"
	case "internal_error":
		return "500"
	}
	if strings.HasSuffix(code, "_not_found") {
		return "404"
	}
	return "400"
}

// Build merges the live route table with the described operations into a
// complete document.
func (r *Registry) Build(options BuildOptions) *Document {
	r.mu.Lock()
	operations := append([]Operation(nil), r.operations...)
	schemeName, scheme := r.schemeName, r.scheme
	r.mu.Unlock()

	described := make(map[string]Operation, len(operations))
	for _, op := range operations {
		described[op.Method+" "+op.Path] = op
	}

	document := &Document{
		OpenAPI: "3.0.3",
		Info:    options.Info,
		Paths:   map[string]*PathItem{},
		Components: &Components{
			Schemas: map[string]*Schema{},
		},
	}
	for _, server := range options.Servers {
		document.Servers = append(document.Servers, Server{URL: server})
	}
	defs := document.Components.Schemas
	defs["Error"] = errorComponent()
	defs["Meta"] = metaComponent()

	securityUsed := false
	routes := append([]Route(nil), options.Routes...)
	sort.Slice(routes, func(a, b int) bool {
		if routes[a].Path == routes[b].Path {
			return routes[a].Method < routes[b].Method
		}
		return routes[a].Path < routes[b].Path
	})

	for _, route := range routes {
		if excluded(route.Path, options) {
			continue
		}
		op, has := described[route.Method+" "+route.Path]
		if has && op.Hidden {
			continue
		}
		specPath, pathParams := convertPath(route.Path)
		item := document.Paths[specPath]
		if item == nil {
			item = &PathItem{}
			document.Paths[specPath] = item
		}
		var object *OperationObject
		if has {
			object = r.operationObject(op, defs)
			if op.Security {
				securityUsed = true
				object.Security = []map[string][]string{{schemeName: {}}}
			}
		} else {
			object = defaultOperation(route)
		}
		for _, param := range pathParams {
			object.Parameters = append([]Parameter{{
				Name: param, In: "path", Required: true,
				Schema: &Schema{Type: "string"},
			}}, object.Parameters...)
		}
		attach(item, route.Method, object)
	}

	if securityUsed {
		document.Components.SecuritySchemes = map[string]*SecurityScheme{schemeName: &scheme}
	}
	return document
}

// excluded performs this package operation.
func excluded(path string, options BuildOptions) bool {
	for _, exact := range options.ExcludePaths {
		if path == exact {
			return true
		}
	}
	for _, prefix := range options.ExcludePrefixes {
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// convertPath rewrites gin params (:id, *rest) to OpenAPI ({id}, {rest}).
func convertPath(path string) (string, []string) {
	segments := strings.Split(path, "/")
	var params []string
	for index, segment := range segments {
		if strings.HasPrefix(segment, ":") || strings.HasPrefix(segment, "*") {
			name := segment[1:]
			params = append(params, name)
			segments[index] = "{" + name + "}"
		}
	}
	return strings.Join(segments, "/"), params
}

// defaultTag performs this package operation.
func defaultTag(path string) string {
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(segments) >= 3 && segments[0] == "api" && strings.HasPrefix(segments[1], "v") {
		return segments[2]
	}
	if segments[0] != "" {
		return segments[0]
	}
	return "default"
}

// defaultOperation performs this package operation.
func defaultOperation(route Route) *OperationObject {
	return &OperationObject{
		Summary: route.Method + " " + route.Path,
		Tags:    []string{defaultTag(route.Path)},
		Responses: map[string]*Response{
			"200": {
				Description: "Success",
				Content: map[string]MediaType{"application/json": {Schema: &Schema{
					Type:       "object",
					Properties: map[string]*Schema{"data": {}},
				}}},
			},
			"500": errorResponse("Internal error", "internal_error"),
		},
	}
}

// operationObject performs this package operation.
func (r *Registry) operationObject(op Operation, defs map[string]*Schema) *OperationObject {
	object := &OperationObject{
		Summary:     op.Summary,
		Description: op.Description,
		Tags:        op.Tags,
		Responses:   map[string]*Response{},
	}
	if len(object.Tags) == 0 {
		object.Tags = []string{defaultTag(op.Path)}
	}
	if op.Request != nil {
		object.RequestBody = &RequestBody{
			Required: true,
			Content:  map[string]MediaType{"application/json": {Schema: schemaFor(op.Request, defs)}},
		}
	}
	if op.Query != nil {
		object.Parameters = append(object.Parameters, queryParameters(op.Query, defs)...)
	}
	for _, filter := range op.Filters {
		object.Parameters = append(object.Parameters, Parameter{
			Name: "filter[" + filter + "]", In: "query",
			Description: "Filter results by " + filter + ".",
			Schema:      &Schema{Type: "string"},
		})
	}
	if len(op.Sorts) > 0 {
		object.Parameters = append(object.Parameters, Parameter{
			Name: "sort", In: "query",
			Description: "Comma-separated sort fields; prefix with - for descending. Allowed: " + strings.Join(op.Sorts, ", ") + ".",
			Schema:      &Schema{Type: "string"},
		})
	}
	if op.Paginated || op.List {
		object.Parameters = append(object.Parameters,
			Parameter{Name: "page", In: "query", Schema: &Schema{Type: "integer"}, Description: "Page number, starting at 1."},
			Parameter{Name: "per_page", In: "query", Schema: &Schema{Type: "integer"}, Description: "Items per page."},
		)
	}

	status := op.Status
	if status == 0 {
		status = http.StatusOK
	}
	if status == http.StatusNoContent {
		object.Responses["204"] = &Response{Description: "No content"}
	} else {
		var dataSchema *Schema
		if op.Response != nil {
			dataSchema = schemaFor(op.Response, defs)
		} else {
			dataSchema = &Schema{}
		}
		envelope := &Schema{Type: "object", Properties: map[string]*Schema{}}
		if op.List {
			envelope.Properties["data"] = &Schema{Type: "array", Items: dataSchema}
			envelope.Properties["meta"] = &Schema{Ref: "#/components/schemas/Meta"}
		} else {
			envelope.Properties["data"] = dataSchema
		}
		object.Responses[itoa(status)] = &Response{
			Description: http.StatusText(status),
			Content:     map[string]MediaType{"application/json": {Schema: envelope}},
		}
	}

	byStatus := map[string][]string{"500": {"internal_error"}}
	if op.Request != nil {
		byStatus["400"] = append(byStatus["400"], "invalid_json")
		byStatus["422"] = append(byStatus["422"], "validation_failed")
	}
	if len(op.Filters) > 0 || len(op.Sorts) > 0 || op.Paginated || op.List || op.Query != nil {
		byStatus["400"] = append(byStatus["400"], "invalid_query")
	}
	if op.Security {
		byStatus["401"] = append(byStatus["401"], "missing_token", "invalid_token")
	}
	for _, code := range op.ErrorCodes {
		status := errorStatus(code)
		byStatus[status] = append(byStatus[status], code)
	}
	for status, codes := range byStatus {
		object.Responses[status] = errorResponse("Error codes: "+strings.Join(dedupe(codes), ", "), codes...)
	}
	return object
}

// queryParameters converts a struct's form-tagged fields into parameters.
func queryParameters(value any, defs map[string]*Schema) []Parameter {
	t := reflect.TypeOf(value)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var parameters []Parameter
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("form"), ",")
		if name == "" || name == "-" {
			continue
		}
		schema := schemaForType(field.Type, defs, map[reflect.Type]string{})
		required := strings.Contains(","+field.Tag.Get("validate")+",", ",required,")
		parameters = append(parameters, Parameter{Name: name, In: "query", Required: required, Schema: schema})
	}
	return parameters
}

// errorResponse performs this package operation.
func errorResponse(description string, _ ...string) *Response {
	return &Response{
		Description: description,
		Content: map[string]MediaType{"application/json": {Schema: &Schema{
			Ref: "#/components/schemas/Error",
		}}},
	}
}

// errorComponent performs this package operation.
func errorComponent() *Schema {
	return &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"error": {
				Type: "object",
				Properties: map[string]*Schema{
					"code":       {Type: "string"},
					"message":    {Type: "string"},
					"details":    {},
					"request_id": {Type: "string"},
				},
				Required: []string{"code", "message"},
			},
		},
	}
}

// metaComponent performs this package operation.
func metaComponent() *Schema {
	return &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"page":        {Type: "integer"},
			"per_page":    {Type: "integer"},
			"total":       {Type: "integer", Format: "int64"},
			"total_pages": {Type: "integer", Format: "int64"},
		},
	}
}

// attach performs this package operation.
func attach(item *PathItem, method string, object *OperationObject) {
	switch method {
	case http.MethodGet:
		item.Get = object
	case http.MethodPost:
		item.Post = object
	case http.MethodPut:
		item.Put = object
	case http.MethodPatch:
		item.Patch = object
	case http.MethodDelete:
		item.Delete = object
	}
}

// dedupe performs this package operation.
func dedupe(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

// itoa performs this package operation.
func itoa(status int) string { return strconv.Itoa(status) }
