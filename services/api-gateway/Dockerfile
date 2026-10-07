FROM golang:1.27
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/build/server ./services/api-gateway/cmd/server
EXPOSE 8080
CMD ["/app/build/server"]
