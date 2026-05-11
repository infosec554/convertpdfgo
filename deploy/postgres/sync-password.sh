#!/usr/bin/env bash
# Idempotent password sync for the postgres container.
#
# Why this exists: the official postgres image only honours POSTGRES_PASSWORD
# on FIRST init (when the data directory is empty). Every subsequent restart
# reads the password from disk and silently ignores the env. If the env or
# compose file ever held a different value than what landed on disk during
# the first init, every later boot leaves the api/worker authenticating with
# the env's password while the actual user has the old one — the failure
# mode we hit three times in one session.
#
# This wrapper:
#   1. Lets the stock entrypoint start postgres normally (initdb on first
#      run, plain start on subsequent runs).
#   2. Waits for postgres to accept local connections (via the Unix socket
#      the postgres image trusts unconditionally for the postgres role).
#   3. Runs `ALTER USER ... WITH PASSWORD` from the env. Idempotent — does
#      nothing if password already matches; recovers the drift case for free.
#   4. Hands control back to postgres so the container behaves identically
#      from the runtime's perspective.
set -e

# Run the real entrypoint in the background so we can interleave a one-shot
# ALTER without sacrificing process-1 status. Pass through all original
# arguments (typically just `postgres`).
docker-entrypoint.sh "$@" &
PG_PID=$!

# Tear postgres down with us if anything goes wrong.
trap 'kill -TERM "$PG_PID" 2>/dev/null || true' INT TERM

# Wait until the local Unix socket is accepting connections. The postgres
# image's pg_hba.conf trusts the socket for the postgres role so we never
# need to know the current password.
until pg_isready -q -U postgres -h /var/run/postgresql 2>/dev/null; do
  sleep 0.5
done

if [ -n "${POSTGRES_PASSWORD:-}" ]; then
  # Quote-escape: any single quote in the password becomes two single quotes
  # inside the SQL literal. The env value itself comes from compose / .env so
  # injection isn't a security concern, but a literal apostrophe would still
  # break the statement.
  escaped=$(printf '%s' "$POSTGRES_PASSWORD" | sed "s/'/''/g")
  psql -q -U postgres -d postgres -h /var/run/postgresql \
    -c "ALTER USER postgres WITH PASSWORD '${escaped}';" >/dev/null && \
    echo "[pwd-sync] postgres password aligned with POSTGRES_PASSWORD env"
fi

# Foreground postgres again so docker sees the right exit code.
wait "$PG_PID"
