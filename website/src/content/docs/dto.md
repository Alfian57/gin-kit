---
title: Request and response DTOs
description: Explicit transport shapes between handlers, services, and clients.
---

Every generated model comes with a dedicated DTO file in `internal/dto`. DTOs
answer two questions explicitly: *what may enter the API* (requests, with
validation tags) and *what may leave it* (responses, with nothing else).

```text
handler  --binds-->  dto.CreateTicketRequest  --into-->  service
service  --returns-->  domain.Ticket  --wrapped by-->  dto.NewTicketResponse
```

Import direction is one-way: `dto` imports `domain`, never the other way
around, and never the service layer. Domain models stay free of HTTP concerns.

## Anatomy of a generated DTO file

`gin-kit generate resource Ticket --fields "title:string,nickname:string?,password:string"`
produces `internal/dto/ticket_dto.go`:

```go
// CreateTicketRequest is the payload accepted when creating a ticket.
type CreateTicketRequest struct {
    Title    string  `json:"title" validate:"required,max=255"`
    Nickname *string `json:"nickname" validate:"omitempty,max=255"`
    Password string  `json:"password" validate:"required,max=255"`
}

// Normalize cleans free-text input in place. Services call it before use.
func (r *CreateTicketRequest) Normalize() {
    r.Title = strings.TrimSpace(r.Title)
    r.Password = strings.TrimSpace(r.Password)
}

// TicketResponse is the representation of a ticket returned to API
// clients. Credential-like fields (password) are deliberately excluded;
// add them back only if they are safe to expose.
type TicketResponse struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    Nickname  *string   `json:"nickname"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func NewTicketResponse(entity domain.Ticket) TicketResponse { ... }
func NewTicketResponseList(entities []domain.Ticket) []TicketResponse { ... }
```

Four properties are guaranteed by the generator:

- **Create and update requests are separate types.** They start identical and
  are free to diverge — the most common evolution in real APIs.
- **Validation tags follow nullability.** `title:string` validates as
  `required,max=255`; `nickname:string?` as `omitempty,max=255`; nullable
  text fields carry no constraint.
- **Credential-like fields never reach responses.** Field names containing
  `password`, `secret`, `token`, or `hash` are excluded from the response
  type, with a comment explaining why.
- **No persistence tags.** Responses carry only `json` tags — the wire format
  cannot silently change because a database column did.

## How the layers use DTOs

Handlers bind and wrap; services normalize and decide; mappers are explicit:

```go
// handler
request, ok := httpx.BindJSON[dto.CreateTicketRequest](c)
if !ok {
    return // 422 with field details already written
}
created, err := tickets.Create(c.Request.Context(), request)
...
httpx.Created(c, dto.NewTicketResponse(created))

// service
func (s *TicketService) Create(ctx context.Context, request dto.CreateTicketRequest) (domain.Ticket, error) {
    request.Normalize()
    entity := domain.Ticket{ID: uuid.NewString(), Title: request.Title, ...}
    ...
    return entity, nil
}
```

Validation happens once, at bind time — services trust their inputs and
return domain values, staying transport-agnostic. In the framework edition,
the same DTO types feed the [auto-generated OpenAPI docs](/gin-kit/api-docs/)
through `Describe`, so the spec and the wire format come from one source.

The auth scaffold follows the same layout: `RegisterRequest`, `LoginRequest`,
`RefreshRequest`, `UserResponse`, `TokenResponse`, and `AuthResponse` live in
`internal/dto/auth_dto.go`, and `/api/v1/me` returns a `UserResponse` that
excludes the password hash by construction.

## Generating DTOs for existing models

`generate resource` includes the DTO file automatically. For a model that
already exists, render just the DTOs:

```bash
gin-kit generate dto Ticket --fields "title:string,nickname:string?"
```

The `--fields` list must match the domain model. See
[CLI and generators](/gin-kit/cli-generators/) for the full grammar.
