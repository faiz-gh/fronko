# Build Stage
FROM node:24-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Serve Stage
FROM nginx:alpine
# PORT: where nginx listens. BACKEND_UPSTREAM: where it proxies /api and /auth.
# API_URL: public backend URL for the browser; empty proxies through this container.
ENV PORT=3000 \
    BACKEND_UPSTREAM=http://backend:8080 \
    API_URL=
COPY --from=builder /app/build /usr/share/nginx/html
COPY docker/default.conf.template /etc/nginx/templates/default.conf.template
COPY --chmod=755 docker/40-runtime-config.sh /docker-entrypoint.d/40-runtime-config.sh
EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
