#!/bin/sh
# install.sh — bootstrap stub for the jira-tickets skill.
#
# Real install logic lives in pkg/install/install.sh, shared by every skill
# in this monorepo. This stub just exports the three required parameters
# and delegates. See pkg/install/install.sh for the full pipeline.

set -e

SKILL_REPO="${SKILL_REPO:-diegoclair/skills}"
SKILL_NAME="jira-tickets"
SKILL_TAG_PREFIX="jira-v"

export SKILL_NAME SKILL_TAG_PREFIX SKILL_REPO
# Forward any opt-in environment users might have set under the old per-skill
# name so back-compat is preserved.
[ -n "$JIRA_TICKETS_VERSION" ] && export SKILL_VERSION="$JIRA_TICKETS_VERSION"

SHARED_URL="https://raw.githubusercontent.com/$SKILL_REPO/main/pkg/install/install.sh"

# Pipe the shared installer into sh. Env vars exported above propagate to
# the child shell across the pipe (fork/exec preserves environment).
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$SHARED_URL" | sh
elif command -v wget >/dev/null 2>&1; then
  wget -qO- "$SHARED_URL" | sh
else
  echo "error: neither curl nor wget found; install one and retry" >&2
  exit 1
fi
