#!/bin/sh
set -eu

child=""
last=""

cleanup() {
  if [ -n "${child}" ]; then
    kill "${child}" 2>/dev/null || true
  fi
}
trap cleanup INT TERM EXIT

while :; do
  fingerprint=$(find . -type f -name '*.go' -exec sha256sum {} \; | sort | sha256sum)
  if [ "${fingerprint}" != "${last}" ]; then
    if [ -n "${child}" ]; then
      kill "${child}" 2>/dev/null || true
      wait "${child}" 2>/dev/null || true
    fi
    if go build -o /tmp/fantasy-helper-dev ./cmd/server; then
      /tmp/fantasy-helper-dev &
      child=$!
      last=${fingerprint}
    else
      echo "backend build failed; retrying"
    fi
  fi
  sleep 1
done
