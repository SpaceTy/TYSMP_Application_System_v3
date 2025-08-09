FROM golang:1.21-alpine AS build
WORKDIR /src

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Build all binaries
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/discordbot ./discordbot

FROM alpine:3.19
RUN adduser -D -H appuser
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/discordbot /usr/local/bin/discordbot
COPY run.sh /usr/local/bin/run.sh
EXPOSE 8081 8080
USER appuser
ENTRYPOINT ["/usr/local/bin/run.sh"]


