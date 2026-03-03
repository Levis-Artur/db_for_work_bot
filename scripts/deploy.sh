#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

run_as_root() {
  if [ "${EUID:-$(id -u)}" -eq 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

assert_ubuntu_like() {
  if ! need_cmd apt-get; then
    echo "This script currently supports Ubuntu/Debian only (apt-get required)."
    exit 1
  fi
}

install_packages_if_missing() {
  local missing=()
  for pkg in "$@"; do
    if ! dpkg -s "$pkg" >/dev/null 2>&1; then
      missing+=("$pkg")
    fi
  done
  if [ "${#missing[@]}" -eq 0 ]; then
    return
  fi
  run_as_root apt-get update -y
  run_as_root apt-get install -y "${missing[@]}"
}

install_base_packages() {
  install_packages_if_missing ca-certificates curl gnupg git
}

install_docker() {
  if need_cmd docker; then
    return
  fi
  curl -fsSL https://get.docker.com | run_as_root sh
}

ensure_docker_running() {
  run_as_root systemctl enable docker >/dev/null 2>&1 || true
  run_as_root systemctl start docker
}

ensure_compose() {
  if docker compose version >/dev/null 2>&1; then
    return
  fi
  install_packages_if_missing docker-compose-plugin
}

ensure_env_file() {
  if [ -f ".env" ]; then
    return
  fi
  cp .env.example .env
  echo ".env created from .env.example"
  echo "Fill BOT_TOKEN, ACCESS_CODE, ADMIN_USER_ID, WEBHOOK_URL, WEBHOOK_SECRET in .env and run script again."
  exit 1
}

validate_env() {
  if grep -qE '^BOT_TOKEN=123:abc$' .env; then
    echo "BOT_TOKEN is still placeholder in .env"
    exit 1
  fi
  if grep -qE '^ACCESS_CODE=CHANGE_ME$' .env; then
    echo "ACCESS_CODE is still placeholder in .env"
    exit 1
  fi
  if grep -qE '^ADMIN_USER_ID=123456789$' .env; then
    echo "ADMIN_USER_ID is still placeholder in .env"
    exit 1
  fi
}

deploy_stack() {
  docker compose pull db migrate >/dev/null 2>&1 || true
  docker compose up -d --build
}

show_status() {
  docker compose ps
}

main() {
  assert_ubuntu_like
  install_base_packages
  install_docker
  ensure_docker_running
  ensure_compose
  ensure_env_file
  validate_env
  deploy_stack
  show_status
}

main "$@"
