FROM node:22-alpine AS builder

# Upgrade system packages
RUN apk update --no-cache && apk upgrade --no-cache

# Install build dependencies for native modules
RUN apk add --no-cache python3 make g++

WORKDIR /app

COPY package*.json ./
RUN npm install --legacy-peer-deps

COPY . .

# Build-time args are no longer needed for deployment configuration.
# All VITE_* vars are injected at container startup via entrypoint.sh.
# These ARGs are kept only for backward compatibility with CI builds
# that pass --build-arg.
ARG VITE_CHAIN_ID=
ARG VITE_NETWORK_NAME=
ARG VITE_NETWORK_CURRENCY=
ARG VITE_RPC_URL=
ENV VITE_CHAIN_ID=$VITE_CHAIN_ID
ENV VITE_NETWORK_NAME=$VITE_NETWORK_NAME
ENV VITE_NETWORK_CURRENCY=$VITE_NETWORK_CURRENCY
ENV VITE_RPC_URL=$VITE_RPC_URL

RUN npm run build

FROM nginx:alpine@sha256:f46cb72c7df02710e693e863a983ac42f6a9579058a59a35f1ae36c9958e4ce0

# Upgrade system packages
RUN apk update --no-cache && apk upgrade --no-cache

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Create nginx cache/pid directories writable by non-root
RUN mkdir -p /var/cache/nginx /var/run && \
    chown -R nginx:nginx /var/cache/nginx /var/run /usr/share/nginx/html /etc/nginx/conf.d && \
    sed -i 's|/var/run/nginx.pid|/tmp/nginx.pid|' /etc/nginx/nginx.conf

# Copy entrypoint script
COPY deploy/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 80

USER nginx

ENTRYPOINT ["/entrypoint.sh"]
