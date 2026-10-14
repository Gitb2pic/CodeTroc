# Front CodeTroc en mode API : build Vite puis service statique par nginx non privilégié.
FROM node:22-alpine AS build
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts
COPY . .
RUN npm run build:api

FROM nginxinc/nginx-unprivileged:1.29-alpine-slim
# Correctifs de sécurité Alpine publiés depuis la construction de l'image de base.
USER root
RUN apk upgrade --no-cache
USER 101
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 8080
