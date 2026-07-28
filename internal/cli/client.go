// Package cli provides gin-kit cli implementation support.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// loadRuntimeOpenAPI performs this package operation.
func loadRuntimeOpenAPI(root string) (*clientSpec, error) {
	cmd := exec.Command("go", "run", "./cmd/server", "--openapi")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("run server --openapi: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("run server --openapi: %w", err)
	}
	return parseClientSpec(output)
}

// clientSpec defines an implementation type used by this package.
type clientSpec struct {
	// Paths store data used by this type.
	Paths map[string]map[string]clientOp `json:"paths"`
	// Components store data used by this type.
	Components *clientComponents `json:"components,omitempty"`
}

// clientComponents defines an implementation type used by this package.
type clientComponents struct {
	// Schemas store data used by this type.
	Schemas map[string]clientSchema `json:"schemas"`
}

// clientOp defines an implementation type used by this package.
type clientOp struct {
	// OperationID store data used by this type.
	OperationID string `json:"operationId"`
	// Parameters store data used by this type.
	Parameters []clientParam `json:"parameters"`
	// RequestBody store data used by this type.
	RequestBody *clientBody `json:"requestBody,omitempty"`
}

// clientParam defines an implementation type used by this package.
type clientParam struct {
	// Name store data used by this type.
	Name string `json:"name"`
	// In store data used by this type.
	In string `json:"in"`
	// Required store data used by this type.
	Required bool `json:"required"`
	// Schema store data used by this type.
	Schema clientSchema `json:"schema"`
}

// clientBody defines an implementation type used by this package.
type clientBody struct {
	// Required store data used by this type.
	Required bool `json:"required"`
	// Content store data used by this type.
	Content map[string]clientContent `json:"content"`
}

// clientContent defines an implementation type used by this package.
type clientContent struct {
	// Schema store data used by this type.
	Schema clientSchema `json:"schema"`
}

// clientSchema defines an implementation type used by this package.
type clientSchema struct {
	// Ref store data used by this type.
	Ref string `json:"$ref"`
	// Type store data used by this type.
	Type string `json:"type"`
	// Nullable store data used by this type.
	Nullable bool `json:"nullable"`
	// Items store data used by this type.
	Items *clientSchema `json:"items"`
	// Properties store data used by this type.
	Properties map[string]clientSchema `json:"properties"`
	// Enum store data used by this type.
	Enum []string `json:"enum"`
}

// parseClientSpec performs this package operation.
func parseClientSpec(b []byte) (*clientSpec, error) {
	if !json.Valid(b) {
		var v any
		if err := yaml.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		var err error
		b, err = json.Marshal(v)
		if err != nil {
			return nil, err
		}
	}
	var s clientSpec
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse OpenAPI document: %w", err)
	}
	if s.Paths == nil {
		return nil, errors.New("OpenAPI document has no paths")
	}
	return &s, nil
}

// loadClientSpec performs this package operation.
func loadClientSpec(source string) (*clientSpec, error) {
	var b []byte
	var err error
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, e := http.Get(source)
		if e != nil {
			return nil, e
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("OpenAPI URL returned %s", resp.Status)
		}
		b, err = io.ReadAll(resp.Body)
	} else {
		b, err = os.ReadFile(source)
	}
	if err != nil {
		return nil, err
	}
	return parseClientSpec(b)
}

// safeTSName performs this package operation.
func safeTSName(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsLetter(r) || r == '_' || (i > 0 && unicode.IsDigit(r)) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "value"
	}
	return b.String()
}

// methodName performs this package operation.
func methodName(m, p string) string {
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '{' || r == '}' || r == '-' || r == '_' })
	var b strings.Builder
	b.WriteString(strings.ToLower(m))
	for _, p := range parts {
		if p != "" {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return safeTSName(b.String())
}

// tsType performs this package operation.
func tsType(s clientSchema) string {
	if s.Ref != "" {
		return safeTSName(filepath.Base(s.Ref))
	}
	if len(s.Enum) > 0 {
		v := make([]string, len(s.Enum))
		for i, x := range s.Enum {
			v[i] = fmt.Sprintf("%q", x)
		}
		return strings.Join(v, " | ")
	}
	switch s.Type {
	case "string":
		return "string"
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		if s.Items != nil {
			return "Array<" + tsType(*s.Items) + ">"
		}
		return "Array<unknown>"
	case "object":
		if len(s.Properties) > 0 {
			k := make([]string, 0, len(s.Properties))
			for x := range s.Properties {
				k = append(k, x)
			}
			sort.Strings(k)
			f := make([]string, len(k))
			for i, x := range k {
				f[i] = safeTSName(x) + ": " + tsType(s.Properties[x])
			}
			return "{ " + strings.Join(f, "; ") + " }"
		}
		return "Record<string, unknown>"
	}
	return "unknown"
}

// renderTSClient performs this package operation.
func renderTSClient(s *clientSpec) []byte {
	var b strings.Builder
	b.WriteString("// Code generated by gin-kit. DO NOT EDIT.\n\nexport type ApiError = { code: string; message: string; details?: unknown; status: number };\nexport type TokenProvider = () => string | undefined;\n")
	if s.Components != nil {
		n := make([]string, 0, len(s.Components.Schemas))
		for x := range s.Components.Schemas {
			n = append(n, x)
		}
		sort.Strings(n)
		for _, x := range n {
			b.WriteString("export type " + safeTSName(x) + " = " + tsType(s.Components.Schemas[x]) + ";\n")
		}
	}
	b.WriteString("\nexport class ApiClient {\n  constructor(private readonly baseUrl = '', private readonly tokenProvider?: TokenProvider) {}\n")
	paths := make([]string, 0, len(s.Paths))
	for p := range s.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	seen := map[string]int{}
	for _, p := range paths {
		ms := make([]string, 0)
		for m := range s.Paths[p] {
			if isHTTPMethod(m) {
				ms = append(ms, m)
			}
		}
		sort.Strings(ms)
		for _, m := range ms {
			o := s.Paths[p][m]
			n := o.OperationID
			if n == "" {
				n = methodName(m, p)
			}
			seen[n]++
			if seen[n] > 1 {
				n = fmt.Sprintf("%s%d", n, seen[n])
			}
			args := []string{}
			pathParams := []clientParam{}
			queryParams := []clientParam{}
			for _, parameter := range o.Parameters {
				optional := "?"
				if parameter.Required || parameter.In == "path" {
					optional = ""
				}
				args = append(args, safeTSName(parameter.Name)+optional+": "+tsType(parameter.Schema))
				switch parameter.In {
				case "path":
					pathParams = append(pathParams, parameter)
				case "query":
					queryParams = append(queryParams, parameter)
				}
			}
			hasBody := false
			if o.RequestBody != nil {
				if content, ok := o.RequestBody.Content["application/json"]; ok {
					optional := "?"
					if o.RequestBody.Required {
						optional = ""
					}
					args = append(args, "body"+optional+": "+tsType(content.Schema))
					hasBody = true
				}
			}
			b.WriteString("  async " + safeTSName(n) + "(" + strings.Join(args, ", ") + "): Promise<unknown> {\n")
			pathExpr := fmt.Sprintf("%q", p)
			for _, parameter := range pathParams {
				name := safeTSName(parameter.Name)
				pathExpr += `.replace(` + fmt.Sprintf("%q", "{"+parameter.Name+"}") + `, encodeURIComponent(String(` + name + `)))`
			}
			b.WriteString("    let path = " + pathExpr + ";\n")
			if len(queryParams) > 0 {
				b.WriteString("    const query = new URLSearchParams();\n")
				for _, parameter := range queryParams {
					name := safeTSName(parameter.Name)
					b.WriteString("    if (" + name + " !== undefined) query.set(" + fmt.Sprintf("%q", parameter.Name) + ", String(" + name + "));\n")
				}
				b.WriteString("    const queryString = query.toString(); if (queryString) path += '?' + queryString;\n")
			}
			bodyArg := "undefined"
			if hasBody {
				bodyArg = "body"
			}
			b.WriteString("    return this.request('" + strings.ToUpper(m) + "', path, " + bodyArg + ");\n  }\n")
		}
	}
	b.WriteString(`  private async request(method: string, path: string, body?: unknown): Promise<unknown> {
    const headers: Record<string,string> = {"Accept":"application/json"};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    const token = this.tokenProvider?.(); if (token) headers["Authorization"] = "Bearer " + token;
    const response = await fetch(this.baseUrl + path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)});
    const body = await response.json().catch(() => undefined);
    if (!response.ok) throw {...(body ?? {}), status: response.status} as ApiError;
    return body;
  }
}
`)
	return []byte(b.String())
}

// isHTTPMethod performs this package operation.
func isHTTPMethod(s string) bool {
	switch strings.ToLower(s) {
	case "get", "post", "put", "patch", "delete", "head", "options", "trace":
		return true
	}
	return false
}
