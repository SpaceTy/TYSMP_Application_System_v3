#!/bin/sh
set -eu

echo "starting container processes..."

if [ "${DISCORD_BOT_TOKEN:-}" != "" ] && [ "${DISCORD_GUILD_ID:-}" != "" ]; then
  echo "launching discordbot on :8080"
  # Ensure line-buffered output and prefix for clarity
  ( /usr/local/bin/discordbot 2>&1 | sed -u 's/^/[discordbot] /' ) &
else
  echo "[discordbot] skipped (missing DISCORD_BOT_TOKEN or DISCORD_GUILD_ID)" >&2
fi

echo "launching api on :${PORT:-8081}"
exec /usr/local/bin/api 2>&1


