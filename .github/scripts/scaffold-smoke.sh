#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(git rev-parse --show-toplevel)"
mode="${GINKIT_MODE:?GINKIT_MODE is required}"
database="${GINKIT_DATABASE:?GINKIT_DATABASE is required}"
orm="${GINKIT_ORM:?GINKIT_ORM is required}"
auth="${GINKIT_AUTH:-false}"
edition="${GINKIT_EDITION:-framework}"
framework_version="${GINKIT_FRAMEWORK_VERSION:-0.3.0}"
project_name="${GINKIT_PROJECT_NAME:-matrixapp}"
runner_temp="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"
cli_path="${GINKIT_CLI_PATH:-$runner_temp/ginkit}"
workdir="$(mktemp -d "${TMPDIR:-/tmp}/ginkit-scaffold.XXXXXX")"
project_dir="$workdir/$project_name"

cleanup() {
  if [[ "$database" != "sqlite" && -f "$project_dir/docker/docker-compose.yml" ]]; then
    (cd "$project_dir" && docker compose -f docker/docker-compose.yml down -v --remove-orphans) || true
  fi
  rm -rf "$workdir"
}
trap cleanup EXIT

if [[ ! -x "$cli_path" ]]; then
  mkdir -p "$(dirname "$cli_path")"
  (cd "$repo_root" && go build -trimpath -o "$cli_path" ./cmd/ginkit)
fi

args=(
  new "$project_name"
  --non-interactive
  --module "example.com/$project_name"
  --edition "$edition"
  --mode "$mode"
  --database "$database"
  --orm "$orm"
  --docker
)
if [[ "$auth" == "true" ]]; then
  args+=(--auth)
fi
if [[ "$edition" == "framework" ]]; then
  args+=(--framework-version "$framework_version" --framework-replace "$repo_root")
fi

(cd "$workdir" && "$cli_path" "${args[@]}")

if [[ "$database" == "postgres" ]]; then
  export GINKIT_DB_PORT="${GINKIT_DB_PORT:-$((20000 + RANDOM % 10000))}"
  export DATABASE_URL="postgres://postgres:postgres@127.0.0.1:$GINKIT_DB_PORT/$project_name?sslmode=disable"
elif [[ "$database" == "mysql" || "$database" == "mariadb" ]]; then
  export GINKIT_DB_PORT="${GINKIT_DB_PORT:-$((30000 + RANDOM % 10000))}"
  export DATABASE_URL="root:root@tcp(127.0.0.1:$GINKIT_DB_PORT)/$project_name?parseTime=true"
else
  export DATABASE_URL="$project_dir/.ginkit-test.sqlite"
fi

if [[ "$database" != "sqlite" ]]; then
  cp "$project_dir/.env.example" "$project_dir/.env"
  started=false
  for attempt in 1 2 3; do
    if (cd "$project_dir" && docker compose -f docker/docker-compose.yml up -d database); then
      started=true
      break
    fi
    sleep $((attempt * 10))
  done
  if [[ "$started" != "true" ]]; then
    echo "database container failed to start after retries" >&2
    exit 1
  fi
  for _ in {1..60}; do
    if (echo >/dev/tcp/127.0.0.1/"$GINKIT_DB_PORT") >/dev/null 2>&1; then
      break
    fi
    sleep 2
  done
  (echo >/dev/tcp/127.0.0.1/"$GINKIT_DB_PORT") >/dev/null 2>&1
fi

(
  cd "$project_dir"
  go mod tidy
  go test ./...
  go vet ./...
  go build ./...
  if [[ "$mode" == "ui" ]]; then
    npm install --no-audit --no-fund
    npm run build
  fi
  "$cli_path" check
  go run ./cmd/migrate up
  go run ./cmd/migrate status
)
