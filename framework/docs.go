package framework

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/Alfian57/gin-kit/framework/httpx"
	"github.com/Alfian57/gin-kit/framework/openapi"
	"github.com/gin-gonic/gin"
)

// swaggerPage serves Swagger UI from the pinned CDN build, mirroring the
// starter edition's /docs page.
const swaggerPage = `<!doctype html><html><head><title>API Docs</title><meta charset="utf-8"><link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"></head><body><div id="swagger-ui"></div><script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script><script>window.onload=()=>SwaggerUIBundle({url:%q,dom_id:"#swagger-ui"})</script></body></html>`

// docsCSP relaxes the UI-mode Content-Security-Policy for the Swagger UI
// page only; the strict default blocks the CDN assets and bootstrap script.
const docsCSP = "default-src 'self'; script-src 'self' 'unsafe-inline' https://unpkg.com; style-src 'self' 'unsafe-inline' https://unpkg.com; img-src 'self' data: https://unpkg.com; connect-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'"

// mountDocs registers the OpenAPI spec endpoint and the Swagger UI page. The
// spec is built lazily on first request so it captures routes the
// application registers after framework.New; routes added after the first
// spec request are not picked up.
func mountDocs(router *gin.Engine, registry *openapi.Registry, options Options) {
	var guards []gin.HandlerFunc
	if options.Docs.BasicAuthUsername != "" {
		guards = append(guards, gin.BasicAuth(gin.Accounts{
			options.Docs.BasicAuthUsername: options.Docs.BasicAuthPassword,
		}))
	}

	var once sync.Once
	var payload []byte
	var buildErr error
	spec := func(c *gin.Context) {
		once.Do(func() {
			routes := make([]openapi.Route, 0, len(router.Routes()))
			for _, route := range router.Routes() {
				routes = append(routes, openapi.Route{Method: route.Method, Path: route.Path})
			}
			excludePaths := []string{options.Docs.Path, options.Docs.SpecPath}
			if options.Metrics.Enabled {
				excludePaths = append(excludePaths, options.Metrics.Path)
			}
			var excludePrefixes []string
			if options.PProf.Enabled {
				excludePrefixes = append(excludePrefixes, options.PProf.Prefix)
			}
			document := registry.Build(openapi.BuildOptions{
				Info: openapi.Info{
					Title:       options.Docs.Title,
					Version:     options.Docs.Version,
					Description: options.Docs.Description,
				},
				Servers:         options.Docs.Servers,
				Routes:          routes,
				ExcludePaths:    excludePaths,
				ExcludePrefixes: excludePrefixes,
			})
			payload, buildErr = json.Marshal(document)
		})
		if buildErr != nil {
			httpx.Handle(c, nil, buildErr)
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
	}
	ui := func(c *gin.Context) {
		c.Header("Content-Security-Policy", docsCSP)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML(options.Docs.SpecPath)))
	}
	router.GET(options.Docs.SpecPath, append(append([]gin.HandlerFunc{}, guards...), spec)...)
	router.GET(options.Docs.Path, append(append([]gin.HandlerFunc{}, guards...), ui)...)
}

func swaggerHTML(specPath string) string {
	return fmt.Sprintf(swaggerPage, specPath)
}
