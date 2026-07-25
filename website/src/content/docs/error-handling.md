---
title: Middleware and errors
description: Make failures useful without leaking internals.
---

The default middleware order is deliberate: request ID, recovery, security
headers, body limits, CORS, rate limiting, logging, then your routes. A panic
becomes a stable `500 internal_error` response and a correlated log entry.

Use `framework.WithErrorMapper` to map a domain error to an HTTP status and
public code. Keep internal causes wrapped for logs, not for clients.

In development, logs include the operation and cause. Production responses
remain stable and concise in every environment. Configure the logger and
redaction policy through `framework.Options`.
