#!/usr/bin/env bash
set -Eeuo pipefail

# Generated smoke projects are not git repositories; skip VCS stamping so the
# smoke behaves identically across CI runners and local sandboxes.
export GOFLAGS="-buildvcs=false${GOFLAGS:+ $GOFLAGS}"

repo_root="$(git rev-parse --show-toplevel)"
mode="${GIN_KIT_MODE:?GIN_KIT_MODE is required}"
database="${GIN_KIT_DATABASE:?GIN_KIT_DATABASE is required}"
orm="${GIN_KIT_ORM:?GIN_KIT_ORM is required}"
auth="${GIN_KIT_AUTH:-false}"
project_type="${GIN_KIT_PROJECT_TYPE:-runtime}"
runtime_version="${GIN_KIT_RUNTIME_VERSION:-0.3.0}"
project_name="${GIN_KIT_PROJECT_NAME:-matrixapp}"
runner_temp="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"
cli_path="${GIN_KIT_CLI_PATH:-$runner_temp/gin-kit}"
workdir="$(mktemp -d "${TMPDIR:-/tmp}/gin-kit-scaffold.XXXXXX")"
project_dir="$workdir/$project_name"

cleanup() {
  if [[ "$database" != "sqlite" && -f "$project_dir/docker/docker-compose.yml" ]]; then
    (cd "$project_dir" && docker compose -f docker/docker-compose.yml down -v --remove-orphans) || true
  fi
  rm -rf "$workdir"
}
trap cleanup EXIT

# Always rebuild so local runs never test a stale CLI binary.
mkdir -p "$(dirname "$cli_path")"
(cd "$repo_root" && go build -trimpath -o "$cli_path" ./cmd/gin-kit)

args=(
  new "$project_name"
  --non-interactive
  --module "example.com/$project_name"
  --project-type "$project_type"
  --mode "$mode"
  --database "$database"
  --orm "$orm"
  --docker
)
if [[ "$auth" == "true" ]]; then
  args+=(--auth)
  # gin-kit routes boots the application, which fail-fasts on a short secret.
  export JWT_SECRET="${JWT_SECRET:-gin-kit-smoke-secret-0123456789abcdef}"
fi
if [[ "$project_type" == "runtime" ]]; then
  args+=(--runtime-version "$runtime_version" --runtime-replace "$repo_root")
fi

(cd "$workdir" && "$cli_path" "${args[@]}")

if [[ "$database" == "postgres" ]]; then
  export GIN_KIT_DB_PORT="${GIN_KIT_DB_PORT:-$((20000 + RANDOM % 10000))}"
  export DATABASE_URL="postgres://postgres:postgres@127.0.0.1:$GIN_KIT_DB_PORT/$project_name?sslmode=disable"
elif [[ "$database" == "mysql" || "$database" == "mariadb" ]]; then
  export GIN_KIT_DB_PORT="${GIN_KIT_DB_PORT:-$((30000 + RANDOM % 10000))}"
  export DATABASE_URL="root:root@tcp(127.0.0.1:$GIN_KIT_DB_PORT)/$project_name?parseTime=true"
else
  export DATABASE_URL="$project_dir/.gin-kit-test.sqlite"
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
    if (echo >/dev/tcp/127.0.0.1/"$GIN_KIT_DB_PORT") >/dev/null 2>&1; then
      break
    fi
    sleep 2
  done
  (echo >/dev/tcp/127.0.0.1/"$GIN_KIT_DB_PORT") >/dev/null 2>&1
fi

# Agent guidance must be emitted and rendered (no leftover template actions).
for guidance in AGENTS.md CLAUDE.md .github/skills/gin-kit-development/SKILL.md; do
  if [[ ! -f "$project_dir/$guidance" ]]; then
    echo "missing agent guidance: $guidance" >&2
    exit 1
  fi
done
if grep -q '{{' "$project_dir/AGENTS.md"; then
  echo "AGENTS.md contains unrendered template actions" >&2
  exit 1
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
  # Generators must produce compiling, passing code in every matrix cell.
  "$cli_path" generate resource SmokeTicket --fields "title:string,done:bool,price:float64,due_at:time"
  "$cli_path" generate resource SmokeArchive --fields "title:string" --soft-delete
  "$cli_path" generate policy SmokeTicket
  "$cli_path" generate middleware SmokeTimer
  "$cli_path" generate seeder Demo
  "$cli_path" generate domain SmokeProfile --fields "email:string,age:int"
  "$cli_path" generate dto SmokeProfile --fields "email:string,age:int"
  "$cli_path" generate factory SmokeProfile --fields "email:string,age:int"
  if [[ "$project_type" == "runtime" ]]; then
    "$cli_path" generate job SmokeJob
    "$cli_path" generate event SmokeEvent
    "$cli_path" generate mail SmokeMail
  fi
  go mod tidy
  go build ./...
  go vet ./...
  go test ./...
  "$cli_path" db seed
  "$cli_path" routes
)
