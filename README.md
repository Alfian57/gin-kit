# GinKit

GinKit is a learning-first Go project toolkit built on [Gin](https://gin-gonic.com/). It creates standalone API or server-rendered UI projects with explicit wiring, practical defaults, and no hidden dependency container.

[![CI](https://github.com/Alfian57/ginkit/actions/workflows/ci.yml/badge.svg)](https://github.com/Alfian57/ginkit/actions/workflows/ci.yml)
[![Security](https://github.com/Alfian57/ginkit/actions/workflows/security.yml/badge.svg)](https://github.com/Alfian57/ginkit/actions/workflows/security.yml)
[![Latest release](https://img.shields.io/github/v/release/Alfian57/ginkit)](https://github.com/Alfian57/ginkit/releases)

## Install

### With Go

```bash
go install github.com/Alfian57/ginkit/cmd/ginkit@latest
```

GinKit requires Go 1.26 or newer when installed from source.

### Prebuilt binaries

Download a binary for Linux, macOS, or Windows from the [release page](https://github.com/Alfian57/ginkit/releases). Verify the downloaded archive before running it:

```bash
sha256sum -c checksums.txt --ignore-missing
gh attestation verify ginkit_<version>_<os>_<arch>.tar.gz -R Alfian57/ginkit
```

Check the installed version:

```bash
ginkit --version
```

## Create a project

```bash
ginkit new my-project
```

The interactive installer asks for the application mode, database, data-access layer, authentication, guided example, and Docker support.

Every generated application starts with production-aware HTTP defaults:
request IDs, security headers, trusted-proxy handling, request body limits,
graceful shutdown, database-backed readiness checks, and endpoint-class rate
limiting. These features stay visible in the generated source so beginners can
inspect and change them.

For scripts, provide complete choices:

```bash
ginkit new my-project \
  --non-interactive \
  --module example.com/my-project \
  --mode api \
  --database sqlite \
  --orm gorm
```

## Project workflow

```bash
ginkit run
ginkit build
ginkit check
ginkit generate resource tasks
ginkit db up
ginkit explain architecture
```

Generated projects remain standalone. The source can be built directly with `go run ./cmd/server` and migrations can be run with `go run ./cmd/migrate up`.

## Design principles

- Keep Gin visible and understandable.
- Prefer constructors and explicit dependencies over magic.
- Keep SQL schema changes versioned and reviewable.
- Make generated code easy to delete, change, and learn from.
- Treat AI agents as collaborators that must follow the same architecture and tests.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md), [AGENTS.md](AGENTS.md), and [SECURITY.md](SECURITY.md) before opening a pull request. Maintainers should also read [docs/releasing.md](docs/releasing.md).

## License

GinKit is released under the MIT License.
