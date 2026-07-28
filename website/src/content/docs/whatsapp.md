---
title: WhatsApp Cloud API
description: Send approved WhatsApp Business templates through Meta's official Cloud API.
---

Runtime projects can send transactional WhatsApp messages directly through the
official Meta Cloud API. The integration is deliberately small: it delivers
approved templates but does not create authentication endpoints, persist OTPs,
or manage verification attempts for an application.

This feature is available to Runtime projects only. Standalone projects remain
dependency-free and do not include the WhatsApp runtime package.

## Configure Meta

In Meta for Developers, add the WhatsApp product to an app, connect a WhatsApp
Business Account and phone number, and create an approved message template.
Create an access token with permission to send messages, then take the phone
number ID from the API setup screen. Keep the access token in your deployment
secret store, never in source control.

Set the following in the deployed environment (or locally in `.env`):

```dotenv
WHATSAPP_DRIVER=cloud
WHATSAPP_ACCESS_TOKEN=your-meta-system-user-access-token
WHATSAPP_PHONE_NUMBER_ID=your-phone-number-id
WHATSAPP_API_VERSION=v25.0
WHATSAPP_TIMEOUT=15s
```

The Graph API version is intentionally explicit. Update it on your own
schedule after verifying the version supported by Meta. At startup, the Cloud
driver rejects incomplete configuration and malformed API version values.

## Send an authentication code

Create a client in application wiring or in the service that owns delivery.
The template name and language are supplied for every send, so they can match
the template approved in the connected WhatsApp Business Account:

```go
client, err := whatsapp.New(cfg.WhatsAppOptions())
if err != nil {
    return err
}
defer client.Close()

message := whatsapp.NewAuthenticationMessage(
    "+62812345678",
    "login_code",
    "id",
    code,
)
if err := client.Send(ctx, message); err != nil {
    return err
}
```

`NewAuthenticationMessage` builds a template message with the code as a text
body parameter. Your approved template must have the matching parameter shape.
The package sends a message; the surrounding OTP policy — generating codes,
expiry, attempt limits, user binding, and verification — remains explicit
application code.

## Send another approved template

Use `NewMessage` for templates with different component structures:

```go
message := whatsapp.NewMessage().
    To("+62812345678").
    Template("order_ready", "id").
    Components(whatsapp.TemplateComponent{
        Type: "body",
        Parameters: []whatsapp.TemplateParameter{
            {Type: "text", Text: order.Number},
        },
    })

err := client.Send(ctx, message)
```

Components are forwarded to Meta's template-message endpoint. Keep their type,
order, and parameter values aligned with the template approved in Meta.
For a non-text parameter, set `TemplateParameter.Data` to the value shape
required by the parameter's type; it is serialized under that type's key.

## Local safety and errors

`WHATSAPP_DRIVER=log` is the default. It performs message validation and writes
only redacted recipient metadata, template name, language, and component count
to structured logs. It never logs access tokens, OTP codes, or template
parameters. This lets development and tests exercise the same service boundary
without sending a real message.

The `cloud` driver uses the standard Go HTTP client to call Meta directly; no
third-party BSP is involved. Cloud delivery failures return a stable error with
the HTTP status but never expose Meta response bodies, tokens, or submitted
message values. Handle that error in your service and return your application's
safe error contract rather than passing it to an HTTP response unchanged.

See [Configuration](/gin-kit/configuration/) for the full environment-variable
reference and [Authentication and security](/gin-kit/auth-security/) for
general application authentication guidance.
