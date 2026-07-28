package openapi

// Minimal OpenAPI 3.0.3 document model, marshaled straight to JSON.

// Document defines an implementation type used by this package.
type Document struct {
	// OpenAPI store data used by this type.
	OpenAPI string `json:"openapi"`
	// Info store data used by this type.
	Info Info `json:"info"`
	// Servers store data used by this type.
	Servers []Server `json:"servers,omitempty"`
	// Paths store data used by this type.
	Paths map[string]*PathItem `json:"paths"`
	// Components store data used by this type.
	Components *Components `json:"components,omitempty"`
}

// Info defines an implementation type used by this package.
type Info struct {
	// Title store data used by this type.
	Title string `json:"title"`
	// Version store data used by this type.
	Version string `json:"version"`
	// Description store data used by this type.
	Description string `json:"description,omitempty"`
}

// Server defines an implementation type used by this package.
type Server struct {
	// URL store data used by this type.
	URL string `json:"url"`
}

// PathItem defines an implementation type used by this package.
type PathItem struct {
	// Get store data used by this type.
	Get *OperationObject `json:"get,omitempty"`
	// Post store data used by this type.
	Post *OperationObject `json:"post,omitempty"`
	// Put store data used by this type.
	Put *OperationObject `json:"put,omitempty"`
	// Patch store data used by this type.
	Patch *OperationObject `json:"patch,omitempty"`
	// Delete store data used by this type.
	Delete *OperationObject `json:"delete,omitempty"`
}

// OperationObject defines an implementation type used by this package.
type OperationObject struct {
	// Summary store data used by this type.
	Summary string `json:"summary,omitempty"`
	// Description store data used by this type.
	Description string `json:"description,omitempty"`
	// Tags store data used by this type.
	Tags []string `json:"tags,omitempty"`
	// Parameters store data used by this type.
	Parameters []Parameter `json:"parameters,omitempty"`
	// RequestBody store data used by this type.
	RequestBody *RequestBody `json:"requestBody,omitempty"`
	// Responses store data used by this type.
	Responses map[string]*Response `json:"responses"`
	// Security store data used by this type.
	Security []map[string][]string `json:"security,omitempty"`
}

// Parameter defines an implementation type used by this package.
type Parameter struct {
	// Name store data used by this type.
	Name string `json:"name"`
	// In store data used by this type.
	In string `json:"in"`
	// Required store data used by this type.
	Required bool `json:"required,omitempty"`
	// Description store data used by this type.
	Description string `json:"description,omitempty"`
	// Schema store data used by this type.
	Schema *Schema `json:"schema,omitempty"`
}

// RequestBody defines an implementation type used by this package.
type RequestBody struct {
	// Required store data used by this type.
	Required bool `json:"required,omitempty"`
	// Content store data used by this type.
	Content map[string]MediaType `json:"content"`
}

// MediaType defines an implementation type used by this package.
type MediaType struct {
	// Schema store data used by this type.
	Schema *Schema `json:"schema,omitempty"`
}

// Response defines an implementation type used by this package.
type Response struct {
	// Description store data used by this type.
	Description string `json:"description"`
	// Content store data used by this type.
	Content map[string]MediaType `json:"content,omitempty"`
}

// Components defines an implementation type used by this package.
type Components struct {
	// Schemas store data used by this type.
	Schemas map[string]*Schema `json:"schemas,omitempty"`
	// SecuritySchemes store data used by this type.
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme defines an implementation type used by this package.
type SecurityScheme struct {
	// Type store data used by this type.
	Type string `json:"type"`
	// Scheme store data used by this type.
	Scheme string `json:"scheme,omitempty"`
	// BearerFormat store data used by this type.
	BearerFormat string `json:"bearerFormat,omitempty"`
	// In store data used by this type.
	In string `json:"in,omitempty"`
	// Name store data used by this type.
	Name string `json:"name,omitempty"`
	// Description store data used by this type.
	Description string `json:"description,omitempty"`
}

// Schema defines an implementation type used by this package.
type Schema struct {
	// Ref store data used by this type.
	Ref string `json:"$ref,omitempty"`
	// Type store data used by this type.
	Type string `json:"type,omitempty"`
	// Format store data used by this type.
	Format string `json:"format,omitempty"`
	// Description store data used by this type.
	Description string `json:"description,omitempty"`
	// Nullable store data used by this type.
	Nullable bool `json:"nullable,omitempty"`
	// Items store data used by this type.
	Items *Schema `json:"items,omitempty"`
	// Properties store data used by this type.
	Properties map[string]*Schema `json:"properties,omitempty"`
	// Required store data used by this type.
	Required []string `json:"required,omitempty"`
	// AdditionalProperties store data used by this type.
	AdditionalProperties *Schema `json:"additionalProperties,omitempty"`
	// AllOf store data used by this type.
	AllOf []*Schema `json:"allOf,omitempty"`
	// MinLength store data used by this type.
	MinLength *int `json:"minLength,omitempty"`
	// MaxLength store data used by this type.
	MaxLength *int `json:"maxLength,omitempty"`
	// Enum store data used by this type.
	Enum []string `json:"enum,omitempty"`
}
