#!/usr/bin/env bash
# Restore a postgres backup from deploy/backup.sh.
#
# DESTRUCTIVE — the dump uses --clean / --if-exists, so applying it
# drops every table in `authservice` before recreating from scratch.
# Refuses to run without an interactive "yes" confirmation.
#
# Usage
# -----
#   deploy/restore.sh latest
#   deploy/restore.sh 20260512-030000      (timestamp from cpdfgo-<stamp>.sql.gz)
#   deploy/restore.sh /path/to/dump.sql.gz
#
# Operationally
# -------------
# Stop the api + worker first so they don't see half-restored schema:
#
#     docker compose --profile full stop api worker
#     deploy/restore.sh latest
#     docker compose --profile full start api worker

set -euo pipefail

DUMP=${1:-}
HOST_DIR=${BACKUP_DIR:-/var/backups/convertpdfgo}

if [ -z "$DUMP" ]; then
    cat <<EOF >&2
usage: $0 <dump.sql.gz | latest | YYYYMMDD-HHMMSS>

Available dumps in ${HOST_DIR}:
EOF
    ls -lh "$HOST_DIR" 2>/dev/null | grep cpdfgo- >&2 || echo "  (none)" >&2
    exit 1
fi

case "$DUMP" in
    latest)
        DUMP="${HOST_DIR}/latest.sql.gz"
        ;;
    /*)
        : # explicit path, leave alone
        ;;
    [0-9]*)
        DUMP="${HOST_DIR}/cpdfgo-${DUMP}.sql.gz"
        ;;
    *)
        DUMP="${HOST_DIR}/${DUMP}"
        ;;
esac

if [ ! -f "$DUMP" ]; then
    echo "[restore] not found: $DUMP" >&2
    exit 2
fi

echo "[restore] target dump: $DUMP"
echo "[restore] size: $(stat -c%s "$DUMP") bytes"
echo "[restore] THIS WILL WIPE every table in authservice."
read -p "[restore] type 'yes' to continue: " ans
if [ "$ans" != "yes" ]; then
    echo "[restore] aborted"
    exit 1
fi

# Wait for postgres readiness so docker exec doesn't race a startup.
docker exec my_postgres bash -c 'until pg_isready -U postgres -q; do sleep 1; done'

echo "[restore] streaming dump into authservice…"
gunzip -c "$DUMP" | docker exec -i my_postgres psql -U postgres -d authservice -v ON_ERROR_STOP=1

echo "[restore] done"
echo "[restore] reminder: bounce api+worker if they were running during the restore."
