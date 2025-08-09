# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/discordbot

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/discordbot /app/discordbot
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/discordbot"]


