FROM golang:1.23.4-alpine3.20 AS build

WORKDIR /var/backend

COPY cmd/main.go main.go
COPY . .


RUN go mod tidy
RUN go build -o main main.go

FROM alpine:edge as prod

RUN apk add bash

COPY --from=build /var/backend/main /app/main
COPY --from=build  /var/backend/ca.crt /app/ca.crt
COPY --from=build  /var/backend/ca.key /app/ca.key
COPY --from=build  /var/backend/cert.key /app/cert.key

WORKDIR /app
EXPOSE 8000
EXPOSE 8080

ENTRYPOINT ./main