FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -o /app/server ./cmd/app/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/config/config.example.yaml ./config/config.yaml
COPY --from=builder /app/database.json .
RUN touch answer_bank.json

EXPOSE 8080

CMD ["./server"]