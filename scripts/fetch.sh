#!/usr/bin/env bash

ROOT=$(git rev-parse --show-toplevel)

fetch() {
  local tool=$1; shift
  local ver=$1; shift

  local ver_cmd=""
  local tool_fetch_cmd=""
  case "$tool" in
    "golangci-lint")
      ver_cmd="${ROOT}/bin/golangci-lint --version 2>/dev/null | cut -d\" \" -f4"
      fetch_cmd="curl -sSfL \"https://raw.githubusercontent.com/golangci/golangci-lint/v${ver}/install.sh\" | sh -s -- -b \"${ROOT}/bin\" \"v${ver}\""
      ;;
    "goreleaser")
      ver_cmd="${ROOT}/bin/goreleaser --version 2>/dev/null | grep 'goreleaser version' | cut -d' ' -f3"
      fetch_cmd="curl -sSfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | sh -s -- -b \"${ROOT}/bin\" -d \"v${ver}\""
      ;;
    *)
      echo "unknown tool $tool"
      return 1
      ;;
  esac

  if [[ "${ver}" != "$(eval ${ver_cmd})" ]]; then
    echo "${tool} missing or not version '${ver}', downloading..."
    eval ${fetch_cmd}
  fi
}
