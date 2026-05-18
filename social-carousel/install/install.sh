#!/bin/sh
# install.sh — bootstrap stub for the social-carousel skill.
#
# Real install logic lives in pkg/install/install.sh, shared by every skill
# in this monorepo. This stub just exports the three required parameters
# and delegates. See pkg/install/install.sh for the full pipeline.

set -e

SKILL_REPO="${SKILL_REPO:-diegoclair/skills}"
SKILL_NAME="social-carousel"
SKILL_TAG_PREFIX="carousel-v"

export SKILL_NAME SKILL_TAG_PREFIX SKILL_REPO

SHARED_URL="https://raw.githubusercontent.com/$SKILL_REPO/main/pkg/install/install.sh"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$SHARED_URL" | sh
elif command -v wget >/dev/null 2>&1; then
  wget -qO- "$SHARED_URL" | sh
else
  echo "error: neither curl nor wget found; install one and retry" >&2
  exit 1
fi
