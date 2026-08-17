# syntax=docker/dockerfile:1

FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /stylelab ./cmd/stylelab

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /stylelab /usr/local/bin/stylelab
ENV STYLELAB_ADDR=:8080
ENV STYLELAB_DATA_DIR=/data
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/stylelab"]
