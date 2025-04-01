FROM golang:1.23.4-alpine3.20 AS build

WORKDIR /var/backend

COPY cmd/main.go main.go
COPY ../go.mod go.mod
COPY ../go.sum go.sum

RUN go mod tidy
RUN go build -o main main.go

FROM alpine:edge as prod

RUN apk add bash

COPY --from=build /var/backend/main /app/main

WORKDIR /app
EXPOSE 8000
EXPOSE 8080

ENTRYPOINT ./main