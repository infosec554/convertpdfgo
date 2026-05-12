#!/usr/bin/env bash
# Postgres dump for convertpdfgo. Runs from the host; talks to the
# `my_postgres` container via `docker exec`.
#
# What this does
# --------------
#   1. pg_dump → ${BACKUP_DIR}/cpdfgo-<UTC-stamp>.sql.gz (gzip -9).
#   2. Refreshes the `latest.sql.gz` symlink so restore.sh can find
#      the newest dump without parsing dates.
#   3. Sanity-checks the dump size (anything under 4 KB is almost
#      certainly a broken pg_dump — fail loud rather than silently
#      ship an empty backup).
#   4. 30-day local retention via `find -mtime +30 -delete`.
#
# What this does NOT do (yet)
# ---------------------------
# Off-host replication. The dumps live on the same disk as postgres'
# own data volume, so they survive postgres-volume corruption / bad
# migrations / accidental DROP, but not a full disk failure or VPS
# loss. For real disaster recovery, layer `rclone` (Backblaze B2 ~
# $0.005/GB or AWS S3) on top of this directory.
#
# Schedule
# --------
# deploy/convertpdfgo-backup.timer fires this script at 03:00 UTC
# every day. Run it manually any time:
#
#     sudo /root/convertpdfgo/deploy/backup.sh

set -euo pipefail

STAMP=$(date -u +%Y%m%d-%H%M%S)
HOST_DIR=${BACKUP_DIR:-/var/backups/convertpdfgo}
DUMP_NAME="cpdfgo-${STAMP}.sql.gz"
DUMP_PATH="${HOST_DIR}/${DUMP_NAME}"

mkdir -p "$HOST_DIR"

echo "[backup] $(date -u +%FT%TZ) — pg_dump → ${DUMP_PATH}"

# --clean + --if-exists make the restore idempotent (it drops the
# existing schema before recreating). --no-owner / --no-privileges
# strip role-specific metadata so the dump replays cleanly even when
# the target cluster's role layout differs (e.g. running locally
# without our sync-password wrapper).
docker exec my_postgres pg_dump \
    -U postgres -d authservice \
    --no-owner --no-privileges --format=plain --clean --if-exists \
  | gzip -9 > "$DUMP_PATH"

SIZE=$(stat -c%s "$DUMP_PATH")
if [ "$SIZE" -lt 4096 ]; then
    echo "[backup] dump too small (${SIZE} bytes) — aborting and removing" >&2
    rm -f "$DUMP_PATH"
    exit 2
fi
echo "[backup] dump size: $(numfmt --to=iec --suffix=B "$SIZE")"

# Atomic-ish symlink refresh: ln -sf overwrites.
ln -sf "$DUMP_NAME" "${HOST_DIR}/latest.sql.gz"

# Retention: 30 daily dumps. Anything older drops.
find "$HOST_DIR" -name "cpdfgo-*.sql.gz" -mtime +30 -delete -print \
    | while read -r f; do echo "[backup] retention: removed $(basename "$f")"; done

echo "[backup] done — next run via systemd timer"
