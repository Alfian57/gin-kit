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

## First steps in a new project

```bash
gin-kit db up                     # apply the initial migrations
gin-kit db seed                   # run the seeder registry
gin-kit routes                    # print the routing table
gin-kit generate resource Ticket --fields "title:string,done:bool"
```

`generate resource` prints the wiring snippet to paste into
`internal/app/app.go`, then `gin-kit db up` applies its migration. In
framework-edition development, interactive API docs are already live at
`http://localhost:8080/docs`. Run `gin-kit check` before committing, and
`gin-kit explain architecture` whenever the layout is unclear.

## The interactive path

Run `gin-kit new ./orders` without `--non-interactive` to choose the edition,
API/UI mode, SQL database, ORM, authentication, example slice, and Docker
support. Use `gin-kit check` before committing.
