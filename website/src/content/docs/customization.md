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
```

See the [Testing guide](/gin-kit/testing/) for request options, session/CSRF
flows, integration databases, factories, and browser tests.

## Feature flags

Framework-edition applications can parse a small set of boolean feature flags
from `FLAGS` and wire the set explicitly:

```go
featureFlags := flags.FromEnv()
taskHandler := handler.NewTaskHandler(taskService, featureFlags)

if featureFlags.Enabled("task-export") {
    router.GET("/api/v1/tasks/export", taskHandler.Export)
}
```

`FLAGS=task-export,new-dashboard` enables both named features. The set is safe
for concurrent reads and updates, so an application-specific admin control can
call `featureFlags.Set("task-export", false)` at runtime. Changes are in-memory
only; gin-kit does not add a global flag registry, persistence, targeting, or
remote evaluation.

Starter-edition projects remain standalone and do not vendor this optional
package. When the same behavior is useful, copy this into
`internal/platform/flags/flags.go` and pass the set through constructors just
like any other dependency:

```go
package flags

import (
    "os"
    "sort"
    "strings"
    "sync"
)

type Set struct {
    mu     sync.RWMutex
    values map[string]bool
}

func New(names ...string) *Set {
    set := &Set{}
    for _, name := range names {
        set.Set(name, true)
    }
    return set
}

func Parse(csv string) *Set { return New(strings.Split(csv, ",")...) }
func FromEnv() *Set         { return Parse(os.Getenv("FLAGS")) }

func (s *Set) Enabled(name string) bool {
    if s == nil {
        return false
    }
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.values[strings.TrimSpace(name)]
}

func (s *Set) Set(name string, on bool) {
    if s == nil {
        return
    }
    name = strings.TrimSpace(name)
    if name == "" {
        return
    }
    s.mu.Lock()
    defer s.mu.Unlock()
    if on {
        if s.values == nil {
            s.values = make(map[string]bool)
        }
        s.values[name] = true
    } else {
        delete(s.values, name)
    }
}

func (s *Set) Names() []string {
    if s == nil {
        return []string{}
    }
    s.mu.RLock()
    defer s.mu.RUnlock()
    names := make([]string, 0, len(s.values))
    for name := range s.values {
        names = append(names, name)
    }
    sort.Strings(names)
    return names
}
```
