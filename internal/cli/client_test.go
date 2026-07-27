package cli

import (
	"strings"
	"testing"
)

func TestRenderTSClientUsesPathQueryAndBodyParameters(t *testing.T) {
	spec, err := parseClientSpec([]byte(`{
		"paths": {"/tickets/{id}": {"patch": {
			"operationId": "updateTicket",
			"parameters": [
				{"name":"id","in":"path","required":true,"schema":{"type":"string"}},
				{"name":"notify","in":"query","schema":{"type":"boolean"}}
			],
			"requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/UpdateTicket"}}}}
		}}},
		"components":{"schemas":{"UpdateTicket":{"type":"object","properties":{"title":{"type":"string"}}}}}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	got := string(renderTSClient(spec))
	for _, want := range []string{
		"async updateTicket(id: string, notify?: boolean, body: UpdateTicket)",
		`.replace("{id}", encodeURIComponent(String(id)))`,
		`query.set("notify", String(notify))`,
		`JSON.stringify(body)`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated client does not contain %q:\n%s", want, got)
		}
	}
}

func TestParseClientSpecAcceptsYAML(t *testing.T) {
	spec, err := parseClientSpec([]byte("openapi: 3.0.3\npaths:\n  /health:\n    get:\n      operationId: health\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := spec.Paths["/health"]; !ok {
		t.Fatal("YAML path was not parsed")
	}
}
