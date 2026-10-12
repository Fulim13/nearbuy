FROM golang:1.27
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/build/server ./services/order-service/cmd/server
# 9000 gRPC, 8000 the DTM branch callbacks.
EXPOSE 9000 8000
CMD ["/app/build/server"]
