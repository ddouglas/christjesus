FROM golang:1.24-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/christjesus ./cmd/christjesus

FROM alpine:3.21
WORKDIR /app
COPY --from=build /out/christjesus /app/christjesus

EXPOSE 8080
ENTRYPOINT ["/app/christjesus", "serve"]
