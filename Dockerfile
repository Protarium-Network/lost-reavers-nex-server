# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/lost-reavers .

FROM alpine:3.20

RUN addgroup -S nex && adduser -S -G nex nex

WORKDIR /app
COPY --from=build /out/lost-reavers /app/lost-reavers

RUN mkdir -p /app/log && chown -R nex:nex /app

USER nex

# PRUDP is UDP. The /udp suffix is not optional: without it Docker publishes
# TCP and the console's packets never arrive.
EXPOSE 60000/udp
EXPOSE 60001/udp

ENTRYPOINT ["/app/lost-reavers"]
