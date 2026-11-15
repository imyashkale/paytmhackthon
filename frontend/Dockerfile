# Build from this folder: docker build -t mcp-frontend frontend
#
# Serves the dashboard and forwards /api/* to the monitor backend.
#   MONITOR_URL  monitor base URL (default http://mcp-monitor, its Kubernetes Service)
FROM node:26-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.29-alpine
ENV MONITOR_URL=http://mcp-monitor
# Only substitute MONITOR_URL so nginx's own $variables are left alone.
ENV NGINX_ENVSUBST_FILTER=^MONITOR_URL$
USER root
RUN rm -f /etc/nginx/conf.d/default.conf
USER 101
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
