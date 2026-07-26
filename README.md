# gin-kit

gin-kit is an opinionated Go framework built on
[Gin](https://gin-gonic.com/). It provides a versioned runtime, an interactive
project CLI, consistent HTTP contracts, SQL integrations, and explicit
application architecture without a reflection-based dependency container.

The default framework edition keeps generic infrastructure in the gin-kit
module while leaving your routes, handlers, services, domains, repositories,
configuration, migrations, and UI fully editable. A standalone starter edition
preserves the source-visible learning experience.

[![CI](https://github.com/Alfian57/gin-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/Alfian57/gin-kit/actions/workflows/ci.yml)
[![Security](https://github.com/Alfian57/gin-kit/actions/workflows/security.yml/badge.svg)](https://github.com/Alfian57/gin-kit/actions/workflows/security.yml)
[![Latest release](https://img.shields.io/github/v/release/Alfian57/gin-kit)](https://github.com/Alfian57/gin-kit/releases)

## Install

### With Go

```bash
go install github.com/Alfian57/gin-kit/cmd/gin-kit@latest
```

gin-kit requires Go 1.26 or newer when installed from source.

### Prebuilt binaries

Download a binary for Linux, macOS, or Windows from the [release page](https://github.com/Alfian57/gin-kit/releases). Verify the downloaded archive before running it:

```bash
sha256sum -c checksums.txt --ignore-missing
gh attestation verify gin-kit_<version>_<os>_<arch>.tar.gz -R Alfian57/gin-kit
```

Check the installed version:

```bash
gin-kit --version
```

## Create a project

```bash
gin-kit new my-project
```

The interactive installer asks for the edition, application mode, database,
data-access layer, authentication, guided example, and Docker support.

Choose the default `framework` edition for a thin application backed by the
versioned gin-kit runtime. Choose `starter` when you want a standalone project
with the infrastructure source included:

```bash
gin-kit new my-project --edition starter
```

For scripts, provide complete choices:

```bash
gin-kit new my-project \
  --non-interactive \
  --edition framework \
  --module example.com/my-project \
  --mode api \
  --database sqlite \
  --orm gorm
```

Every application starts with request IDs, secure recovery, security headers,
trusted-proxy handling, body limits, graceful shutdown, database-backed
readiness checks, and endpoint-class rate limiting. API responses use one
stable envelope, including detailed field-level validation errors.

## Project workflow

```bash
gin-kit run
gin-kit build
gin-kit check
gin-kit generate resource tasks
gin-kit db up
gin-kit explain architecture
```

The generated server can be built directly with `go run ./cmd/server`, and
migrations can be run with `go run ./cmd/migrate up`. Starter projects are
standalone; framework projects pin their gin-kit runtime version and can be built
without the gin-kit CLI installed.

## Design principles

- Keep Gin and standard Go types accessible.
- Prefer constructors and explicit dependencies over magic.
- Hide generic framework plumbing, not application behavior.
- Provide stable response and validation contracts by default.
- Keep SQL schema changes versioned and reviewable.
- Make application-owned generated code easy to delete or change.
- Treat AI agents as collaborators that must follow the same architecture and tests.

## Documentation

The documentation site tracks the next gin-kit release from `main`:
[alfian57.github.io/gin-kit](https://alfian57.github.io/gin-kit/).
Until the next framework release is tagged, the latest GitHub release remains
the stable CLI.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md), [AGENTS.md](AGENTS.md), and [SECURITY.md](SECURITY.md) before opening a pull request. Maintainers should also read [docs/releasing.md](docs/releasing.md).

## License

gin-kit is released under the MIT License.
