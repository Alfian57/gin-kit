---
title: Quickstart
description: Create and run your first gin-kit application.
---

## Requirements

- Go 1.26 or newer
- A SQL database when you choose anything other than SQLite
- Docker (optional, for local database services)

Install the CLI from a release:

```bash
go install github.com/Alfian57/gin-kit/cmd/gin-kit@latest
```

Create an API application with a custom module path:

```bash
gin-kit new ./orders --non-interactive \
  --module example.com/acme/orders \
  --mode api --database sqlite --orm gorm
cd orders
gin-kit run
```

Open `http://localhost:8080/health/live`. The generated `.env.example` lists
the configuration your chosen database and features need.

## The interactive path

Run `gin-kit new ./orders` without `--non-interactive` to choose the edition,
API/UI mode, SQL database, ORM, authentication, example slice, and Docker
support. Use `gin-kit check` before committing.
