FROM oven/bun:1.3-alpine AS builder
WORKDIR /app

COPY package.json bun.lock ./
COPY apps/marketing/package.json ./apps/marketing/
COPY apps/dashboard/package.json ./apps/dashboard/
COPY packages/ui/package.json ./packages/ui/
COPY e2e/package.json ./e2e/
RUN bun install --frozen-lockfile --ignore-scripts

COPY tsconfig.json ./
COPY apps/marketing ./apps/marketing

RUN cd apps/marketing && bun run build

FROM nginx:1.27-alpine
COPY --from=builder /app/apps/marketing/dist /usr/share/nginx/html
RUN printf 'server {\n  listen 80;\n  root /usr/share/nginx/html;\n  index index.html;\n  location / {\n    try_files $uri $uri/ /index.html;\n  }\n}\n' \
  > /etc/nginx/conf.d/default.conf
EXPOSE 80
