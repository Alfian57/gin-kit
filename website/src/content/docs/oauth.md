---
title: OAuth social sign-in
description: Google and GitHub sign-in with explicit, secure browser flows.
---

Add social sign-in when creating a project with both authentication flags:

```bash
gin-kit new social-app --non-interactive \
  --project-type runtime --module example.com/social-app \
  --mode api --database sqlite --orm gorm --auth --oauth
```

`--oauth` requires `--auth`. It generates Google and GitHub routes, an
`oauth_identities` migration, a repository, a service that links identities,
and encrypted browser-session wiring in Runtime and Standalone projects. The
same HTTP contract is generated for API and UI modes.

## Provider setup

Create OAuth credentials with the provider, then register the exact callback
URLs served by your application:

```text
http://localhost:8080/auth/oauth/google/callback
http://localhost:8080/auth/oauth/github/callback
```

For a deployed application, replace the origin with its public HTTPS origin.
Set all three variables for each provider you enable; partial provider
configuration stops startup:

```dotenv
OAUTH_GOOGLE_CLIENT_ID=...
OAUTH_GOOGLE_CLIENT_SECRET=...
OAUTH_GOOGLE_REDIRECT_URL=https://app.example/auth/oauth/google/callback

OAUTH_GITHUB_CLIENT_ID=...
OAUTH_GITHUB_CLIENT_SECRET=...
OAUTH_GITHUB_REDIRECT_URL=https://app.example/auth/oauth/github/callback

OAUTH_SUCCESS_REDIRECT=/
OAUTH_FAILURE_REDIRECT=/sign-in
```

The success and failure targets must be relative paths. This prevents an
environment variable from creating an open redirect. Leaving a provider's
three variables empty simply leaves that provider unavailable; a request to
it redirects to the failure path with a generic flash message.

## Routes and account linking

| Route | Purpose |
| --- | --- |
| `GET /auth/oauth/google` | Start Google sign-in. |
| `GET /auth/oauth/github` | Start GitHub sign-in. |
| `GET /auth/oauth/:provider/callback` | Finish sign-in and redirect. |
| `GET /api/v1/auth/csrf` | Return a CSRF token for a cookie-authenticated API client. |

The callback resolves an existing `(provider, provider_subject)` identity
first. For a new identity, gin-kit links it to a local account only after the
provider supplies a verified email address; otherwise it creates a passwordless
user and identity together. Google identities require a verified OIDC email;
GitHub selects a verified email from its email endpoint. Provider access and
refresh tokens are neither returned, logged, nor stored.

An OAuth-created account has no local password. Add a deliberate password-set
or recovery workflow in your application if you want to offer password login
later.

## Security model

Each browser attempt has a cryptographically random, single-use `state`, a
PKCE S256 verifier, and a ten-minute expiry in the encrypted session. Google
also verifies the OIDC ID-token nonce. The state is removed before the code is
exchanged, so a callback cannot be replayed. Callback failures use stable,
generic codes internally and redirect without exposing provider responses.

Successful callbacks create the local encrypted session. Protect routes that
should accept either that browser session or a Bearer token with
`auth.RequireLogin(tokens)` (Runtime) or `middleware.RequireLogin(secret)`
(Standalone). Keep `RequireAuth` for Bearer-only endpoints such as
machine-to-machine APIs.

Cookie-authenticated requests under `/api/` are CSRF-protected. Fetch
`GET /api/v1/auth/csrf`, read the `X-CSRF-Token` response header, and send it
in that header on state-changing API requests. Bearer-token requests remain
exempt because browsers cannot attach their Authorization header cross-site.

## Custom providers

The runtime keeps registration explicit. Implement `oauth.Provider` and pass
it to `oauth.NewManager`; the provider only returns a verified `Identity`.
There is no global provider registry or hidden dependency injection. The
generated application shows the Google and GitHub constructor calls in
`internal/app/app.go`, which is the place to add another provider deliberately.

See [Authentication and security](/gin-kit/auth-security/) for JWT and API
tokens, and [Sessions and CSRF](/gin-kit/sessions/) for browser-session
details.
