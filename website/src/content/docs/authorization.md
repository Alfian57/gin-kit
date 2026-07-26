---
title: Authorization
description: Explicit allow/deny decisions with one stable 403 contract.
---

Authorization in gin-kit is explicit and allowlist-shaped: policies are plain
structs in `internal/policy` whose methods return an `authz.Decision`, and
nothing is granted implicitly. Framework-edition projects import
`framework/authz`; starter projects vendor the same package as
`internal/platform/authz`. The HTTP contract is identical in both editions.

## The Decision contract

```go
type Decision struct {
    Allowed bool
    Reason  string // internal; logged, never serialized
}

authz.Allow()               // grant the action
authz.Deny("not the owner") // reject it with an internal reason
```

The zero value denies, so a forgotten rule fails closed. A denied decision
always renders the same envelope — status `403` with the stable code
`forbidden` and the message "You are not allowed to perform this action."
The deny reason exists for operators: it is logged and carried as the error
cause, and never reaches the response body.

## Generating a policy

```text
gin-kit generate policy Ticket
```

writes `internal/policy/ticket_policy.go` with `CanView`, `CanCreate`,
`CanUpdate`, and `CanDelete` methods plus a table test. Each method ships a
placeholder rule (deny empty subjects, allow everyone else) and a comment
telling you to replace it with the real ownership or role check. The policy
references `domain.Ticket`, so generate the domain first if it is missing.
The generator works in both editions; starter projects scaffolded before the
authz package existed get `internal/platform/authz` back-filled
automatically.

## Enforcing in handlers

`authz.Authorize` returns `true` and writes nothing when the decision
allows. When it denies, it logs the internal reason, writes the canonical
`403 forbidden` envelope, aborts the request, and returns `false`:

```go
p := policy.NewTicketPolicy()
if !authz.Authorize(c, p.CanUpdate(c.Request.Context(), auth.UserID(c), ticket)) {
    return
}
```

Starter projects read the subject from wherever their middleware stored it:

```go
if !authz.Authorize(c, p.CanUpdate(c.Request.Context(), c.GetString("user_id"), ticket)) {
    return
}
```

## Denying from services

When the check belongs in the service layer, convert the decision into an
error with `Decision.Err()`. It returns `nil` when allowed; otherwise a
`*httpx.Error` that `httpx.DefaultMapper` passes through unchanged, so a
denied service call renders the same `403 forbidden` envelope with the
reason preserved as the wrapped cause for logs:

```go
func (s *TicketService) Update(ctx context.Context, subjectID, id string, request dto.UpdateTicketRequest) (domain.Ticket, error) {
    ticket, err := s.repository.Find(ctx, id)
    if err != nil {
        return domain.Ticket{}, err
    }
    if err := s.policy.CanUpdate(ctx, subjectID, ticket).Err(); err != nil {
        return domain.Ticket{}, err
    }
    // ...
}
```

## Works without authentication

Policies do not require the `--auth` vertical — the subject is just a
string. With [authentication](/gin-kit/auth-security/) enabled, use
`auth.UserID(c)` (framework) or the `user_id` context value (starter);
without it, pass an API-key ID, a tenant, or whatever identifier your own
middleware resolves. The generated placeholder rules deny empty subjects
either way.
