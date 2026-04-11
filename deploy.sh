#!/bin/bash
# LiteChat deployment script
set -euo pipefail

PROJECT_NAME="LiteChat"
CONTAINER_NAME="litechat"

log() {
  echo ">>> $1"
}

fail() {
  echo "!!! $1" >&2
  exit 1
}

run_as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    fail "This step requires root privileges. Please run the script as root or install sudo first."
  fi
}

ensure_command() {
  if command -v "$1" >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

install_basic_tools() {
  if ensure_command curl; then
    return 0
  fi

  log "curl not found, installing prerequisite packages..."

  if ensure_command apt-get; then
    run_as_root apt-get update
    run_as_root apt-get install -y curl ca-certificates
  elif ensure_command dnf; then
    run_as_root dnf install -y curl ca-certificates
  elif ensure_command yum; then
    run_as_root yum install -y curl ca-certificates
  elif ensure_command apk; then
    run_as_root apk add --no-cache curl ca-certificates
  else
    fail "Unsupported package manager. Please install curl manually and rerun deploy.sh."
  fi
}

install_docker() {
  log "Docker not found, installing Docker..."
  install_basic_tools

  curl -fsSL https://get.docker.com | run_as_root sh

  if ensure_command systemctl; then
    run_as_root systemctl enable --now docker
  elif ensure_command service; then
    run_as_root service docker start || true
  fi

  if [ "$(id -u)" -ne 0 ] && ensure_command usermod; then
    run_as_root usermod -aG docker "$USER" || true
  fi

  ensure_command docker || fail "Docker installation failed."
}

ensure_docker() {
  if ensure_command docker; then
    return 0
  fi
  install_docker
}

setup_docker_commands() {
  if docker info >/dev/null 2>&1; then
    DOCKER_CMD=(docker)
  elif ensure_command sudo && sudo docker info >/dev/null 2>&1; then
    DOCKER_CMD=(sudo docker)
  else
    fail "Docker is installed but not accessible. Please check the Docker daemon status."
  fi

  if "${DOCKER_CMD[@]}" compose version >/dev/null 2>&1; then
    COMPOSE_CMD=("${DOCKER_CMD[@]}" compose)
  elif ensure_command docker-compose; then
    if [ "${DOCKER_CMD[0]}" = "sudo" ]; then
      COMPOSE_CMD=(sudo docker-compose)
    else
      COMPOSE_CMD=(docker-compose)
    fi
  else
    fail "Docker Compose is not available. Please install Docker Compose and rerun deploy.sh."
  fi
}

log "Checking Docker environment..."
ensure_docker
setup_docker_commands

log "Pulling latest code..."
git pull

log "Stopping old containers..."
"${COMPOSE_CMD[@]}" down --remove-orphans || true
"${DOCKER_CMD[@]}" rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true

log "Building images..."
"${COMPOSE_CMD[@]}" build

log "Starting containers..."
"${COMPOSE_CMD[@]}" up -d

log "${PROJECT_NAME} deployed successfully."
"${DOCKER_CMD[@]}" ps --filter "name=${CONTAINER_NAME}"

if [ "$(id -u)" -ne 0 ] && ! docker info >/dev/null 2>&1; then
  echo
  echo "Note: Docker has been installed for this server."
  echo "You may need to log out and back in once before using 'docker' without sudo."
fi
