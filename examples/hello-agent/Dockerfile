FROM docker.m.daocloud.io/library/golang:1.25-alpine AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o agent main.go

FROM docker.m.daocloud.io/library/alpine:latest

RUN apk add --no-cache ca-certificates
WORKDIR /
COPY --from=builder /workspace/agent .

USER 65532:65532
EXPOSE 9001
ENTRYPOINT ["./agent"]
