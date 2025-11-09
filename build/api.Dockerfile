FROM golang:1.24-alpine AS build
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/api ./cmd/api

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /bin/api /bin/api
COPY api /app/api               
COPY .env.example /app/.env     
EXPOSE 8080
USER 65532:65532                
ENTRYPOINT ["/bin/api"]
