FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /el-mcp-server .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /el-mcp-server /el-mcp-server

EXPOSE 8080

ENTRYPOINT ["/el-mcp-server"]
CMD ["-transport", "http", "-addr", ":8080"]
