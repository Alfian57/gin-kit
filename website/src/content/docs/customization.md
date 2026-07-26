---
title: Customizing the runtime
description: Keep the defaults and change what your application needs.
---

```go
app, err := framework.New(framework.Options{
    Environment: os.Getenv("APP_ENV"),
    ErrorMapper: mapDomainError,
    HTTP: framework.HTTPOptions{
        Address:        ":8080",
        TrustedProxies: []string{"10.0.0.0/8"},
        CORSOrigins:    []string{"https://app.example.com"},
    },
})
if err != nil {
    return err
}

app.Use(myMiddleware())
app.OnShutdown(closeMetrics)
return app.Run(ctx)
```

The raw `*gin.Engine` and selected database handles remain available. Add
middleware, readiness checks, validation rules, translations, error mappers,
and shutdown hooks through options and interfaces. If the framework itself must
change, fork the module and use a standard Go `replace` directive.

## Testing handlers

The `framework/apptest` package removes the recorder boilerplate from handler
tests while keeping assertions in plain Go:

```go
app := apptest.New(t, framework.Options{})
app.Router().POST("/tasks", createTask)

var task Task
app.POST("/tasks", newTask).RequireStatus(http.StatusCreated).Data(&task)

failure := app.GET("/tasks/absent").RequireStatus(http.StatusNotFound).Err()
// failure.Code == "not_found"
```

`Do` sends JSON bodies and custom headers; `Data` decodes the `data` envelope
field; `Err` decodes the error envelope.
