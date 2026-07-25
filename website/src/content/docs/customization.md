---
title: Customizing the runtime
description: Keep the defaults and change what your application needs.
---

```go
app, err := framework.New(
    framework.WithName("orders"),
    framework.WithEnv(config.Load()),
    framework.WithErrorMapper(mapDomainError),
)
if err != nil {
    return err
}

app.Router().Use(myMiddleware())
app.OnShutdown(closeMetrics)
return app.Run(ctx)
```

The raw `*gin.Engine` and selected database handles remain available. Add
middleware, readiness checks, validation rules, translations, error mappers,
and shutdown hooks through options and interfaces. If the framework itself must
change, fork the module and use a standard Go `replace` directive.
