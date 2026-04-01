FROM node:22-alpine AS builder

# Upgrade system packages
RUN apk update --no-cache && apk upgrade --no-cache

# Install build dependencies for native modules
RUN apk add --no-cache python3 make g++

WORKDIR /app

COPY package*.json ./
RUN npm install --legacy-peer-deps

COPY . .

ARG VITE_CHAIN_ID=1001
ARG VITE_NETWORK_NAME=Gateway
ARG VITE_NETWORK_CURRENCY=ETH
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
    chown -R nginx:nginx /var/cache/nginx /var/run /usr/share/nginx/html /etc/nginx/conf.d

EXPOSE 80

USER nginx

CMD ["nginx", "-g", "daemon off;"]
