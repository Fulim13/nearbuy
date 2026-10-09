FROM golang:1.27
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/build/server ./services/shop-service/cmd/server
EXPOSE 9001
CMD ["/app/build/server"]
