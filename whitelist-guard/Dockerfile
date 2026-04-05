FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /whitelist-guard .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /whitelist-guard /whitelist-guard
EXPOSE 8080
ENTRYPOINT ["/whitelist-guard"]
