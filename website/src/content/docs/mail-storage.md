---
title: Mail and file storage
description: Send email and store files with swappable drivers.
---

## Mail

Build the mailer from configuration in your application code:

```go
mailer, err := mail.New(cfg.MailOptions())

message := mail.NewMessage().
    To(user.Email).
    Subject("Welcome!").
    HTMLTemplate(templates, "welcome.html", map[string]any{"Name": user.Name}).
    Text("Welcome to the app.")

err = mailer.Send(ctx, message)
```

`MAIL_DRIVER=log` (the default) renders the complete MIME message into your
structured logs — no SMTP server needed in development. `MAIL_DRIVER=smtp`
sends through `MAIL_HOST`/`MAIL_PORT` with `MAIL_ENCRYPTION` set to
`starttls` (587, default), `tls` (465), or `none` (25), authenticating when
`MAIL_USERNAME` is set. For a local inbox UI, run
`docker compose --profile mail up` and point `MAIL_HOST=localhost`,
`MAIL_PORT=1025`, `MAIL_ENCRYPTION=none` at Mailpit (web UI on :8025).

Scaffold a typed mailable plus its HTML template with
`gin-kit generate mail <Name>` (runtime project type only).

## File storage

`runtime/storage` puts local and S3-compatible file storage behind one interface:

```go
disk, err := storage.New(cfg.StorageOptions())

err = disk.Put(ctx, "avatars/user-1.png", file, storage.WithContentType("image/png"))
reader, err := disk.Get(ctx, "avatars/user-1.png")
url, err := disk.URL(ctx, "avatars/user-1.png")

// In a handler: store a multipart upload directly.
size, err := storage.SaveUpload(c, disk, "avatar", "avatars/"+userID+".png")
```

`STORAGE_DRIVER=local` (default) confines every path to `STORAGE_LOCAL_ROOT`
— traversal attempts like `../../etc/passwd` are rejected, and writes are
atomic. `STORAGE_DRIVER=s3` works with any S3-compatible store (AWS S3,
MinIO, Cloudflare R2, DigitalOcean Spaces) through `S3_ENDPOINT`, `S3_BUCKET`,
`S3_ACCESS_KEY`/`S3_SECRET_KEY`; `URL()` returns `S3_PUBLIC_BASE_URL` joins
when configured, presigned URLs (`S3_PRESIGN_TTL`) otherwise. For local
object storage, `docker compose --profile storage up` starts MinIO
(`S3_ENDPOINT=localhost:9000`, `S3_USE_PATH_STYLE=true`, `S3_USE_SSL=false`,
credentials `minioadmin`).
