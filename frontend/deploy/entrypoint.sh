#!/bin/sh
# Generate runtime config from environment variables.
# All VITE_* env vars are exposed to the browser via window.__runtimeConfig.
#
# SECURITY: Values are sanitized to prevent XSS. Only VITE_* prefixed
# variables are included — never put secrets in VITE_* env vars.

CONFIG_FILE="/usr/share/nginx/html/config.js"

echo "window.__runtimeConfig = {" > "$CONFIG_FILE"

# Iterate all env vars starting with VITE_
env | grep '^VITE_' | sort | while IFS='=' read -r key value; do
  # Sanitize value to prevent XSS:
  #   1. Escape backslashes first (\ → \\)
  #   2. Escape double quotes (" → \")
  #   3. Remove newlines/carriage returns (break JS string literals)
  #   4. Escape </script> sequences (break out of script tag)
  sanitized=$(printf '%s' "$value" | \
    sed 's/\\/\\\\/g' | \
    sed 's/"/\\"/g' | \
    tr -d '\n\r' | \
    sed 's/<\/script>/<\\\/script>/gi')
  echo "  \"$key\": \"$sanitized\"," >> "$CONFIG_FILE"
done

echo "};" >> "$CONFIG_FILE"

echo "Runtime config generated with $(grep -c '"VITE_' "$CONFIG_FILE") variables"

# Start nginx
exec nginx -g "daemon off;"
