#!/bin/sh
# Generate runtime config from environment variables.
# All VITE_* env vars are exposed to the browser via window.__runtimeConfig.

CONFIG_FILE="/usr/share/nginx/html/config.js"

echo "window.__runtimeConfig = {" > "$CONFIG_FILE"

# Iterate all env vars starting with VITE_
env | grep '^VITE_' | sort | while IFS='=' read -r key value; do
  # Escape double quotes in values
  escaped_value=$(echo "$value" | sed 's/"/\\"/g')
  echo "  \"$key\": \"$escaped_value\"," >> "$CONFIG_FILE"
done

echo "};" >> "$CONFIG_FILE"

echo "Runtime config generated with $(grep -c '"VITE_' "$CONFIG_FILE") variables"

# Start nginx
exec nginx -g "daemon off;"
