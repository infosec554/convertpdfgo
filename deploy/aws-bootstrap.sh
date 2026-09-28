#!/usr/bin/env bash
# One-shot bootstrap for a fresh Ubuntu EC2 instance (eu-north-1).
#
# Takes a blank box to "docker compose up" ready: engine, swap, both
# repos. Deliberately does NOT start anything — .env carries every
# secret and has to land on the host by hand first (see the end).
#
# Usage, as root on the instance:
#   curl -fsSL <raw url>/deploy/aws-bootstrap.sh | sudo bash
# or, after cloning:
#   sudo bash deploy/aws-bootstrap.sh
#
# Safe to re-run — every step checks before acting.

set -euo pipefail

BACKEND_REPO=${BACKEND_REPO:-https://github.com/infosec554/convertpdfgo.git}
FRONTEND_REPO=${FRONTEND_REPO:-https://github.com/infosec554/convertopdffrontend.git}
BACKEND_DIR=/root/convertpdfgo
FRONTEND_DIR=/root/converto   # compose's build.context expects this exact path

log() { printf '\n==> %s\n' "$*"; }

[ "$(id -u)" -eq 0 ] || { echo "root sifatida ishga tushiring (sudo)"; exit 1; }

# ---------------------------------------------------------------------------
log "Paketlar yangilanmoqda"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq ca-certificates curl gnupg git

# ---------------------------------------------------------------------------
# Swap. t3.small has 2 GB of RAM and the stack idles near 0.9 GB; swap is
# what keeps a `docker compose build` or a Gotenberg conversion spike from
# taking the whole box down with the OOM killer. Not a substitute for RAM —
# a safety net under it.
log "Swap sozlanmoqda (4 GB)"
if ! swapon --show | grep -q '/swapfile'; then
  fallocate -l 4G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile >/dev/null
  swapon /swapfile
  grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
  # Prefer RAM, but don't refuse swap so hard that the OOM killer wins first.
  sysctl -qw vm.swappiness=20
  grep -q '^vm.swappiness' /etc/sysctl.conf || echo 'vm.swappiness=20' >> /etc/sysctl.conf
  echo "swap yoqildi"
else
  echo "swap allaqachon bor — o'tkazib yuborildi"
fi

# ---------------------------------------------------------------------------
log "Docker o'rnatilmoqda"
if ! command -v docker >/dev/null 2>&1; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  # `. /etc/os-release` gives VERSION_CODENAME; Docker has no repo for
  # very new Ubuntu releases on day one, so fall back to the LTS that
  # release is built on rather than failing the whole bootstrap.
  . /etc/os-release
  CODENAME=${UBUNTU_CODENAME:-$VERSION_CODENAME}
  if ! curl -fsI "https://download.docker.com/linux/ubuntu/dists/${CODENAME}/Release" >/dev/null 2>&1; then
    echo "Docker repo '${CODENAME}' uchun yo'q — 'noble' (24.04 LTS) ishlatiladi"
    CODENAME=noble
  fi
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io \
    docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
else
  echo "docker allaqachon bor — o'tkazib yuborildi"
fi

# Keep container logs from silently eating the 42 GB root volume.
if [ ! -f /etc/docker/daemon.json ]; then
  mkdir -p /etc/docker
  cat > /etc/docker/daemon.json <<'JSON'
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" }
}
JSON
  systemctl restart docker
  echo "docker log rotation yoqildi"
fi

# ---------------------------------------------------------------------------
log "Repolar klonlanmoqda"
# Private repos can't be cloned from the host without a deploy key. When
# the code is shipped another way (rsync from a workstation, or prebuilt
# images), set SKIP_CLONE=1 and this step stands down.
if [ "${SKIP_CLONE:-0}" = "1" ]; then
  echo "SKIP_CLONE=1 — o'tkazib yuborildi"
else
clone_or_pull() {
  local url=$1 dir=$2
  if [ -d "$dir/.git" ]; then
    echo "$dir allaqachon bor — fetch qilinmoqda"
    git -C "$dir" fetch --quiet origin && git -C "$dir" reset --hard origin/main
  else
    git clone --quiet "$url" "$dir"
    echo "$dir klonlandi"
  fi
}
clone_or_pull "$BACKEND_REPO"  "$BACKEND_DIR"
clone_or_pull "$FRONTEND_REPO" "$FRONTEND_DIR"
fi

# ---------------------------------------------------------------------------
log "Tayyor"
cat <<'NEXT'

Qolgan qadamlar (qo'lda, chunki .env maxfiy):

  1. .env ni shu yerga ko'chiring — lokal kompyuterdan:
       scp -i convertpdfgo.pem .env ubuntu@<IP>:/tmp/.env
       sudo mv /tmp/.env /root/convertpdfgo/.env
       sudo chmod 600 /root/convertpdfgo/.env

  2. DNS: convertpdfgo.com va admin.convertpdfgo.com A yozuvlarini
     shu serverning Elastic IP'siga yo'naltiring. Caddy sertifikat
     ololmaydi, agar DNS hali eski serverni ko'rsatib tursa.

  3. Stack'ni ko'taring:
       cd /root/convertpdfgo
       docker compose --profile full up -d

  4. Holatni tekshiring:
       docker compose --profile full ps
       docker compose --profile full logs -f caddy

NEXT
