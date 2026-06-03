# Build stage
FROM docker.m.daocloud.io/library/golang:1.25-alpine AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o manager cmd/manager/main.go

# Runtime stage
FROM docker.m.daocloud.io/library/alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /

COPY --from=builder /workspace/manager .

USER 65532:65532

EXPOSE 8080 8081 8082

ENTRYPOINT ["./manager"]
