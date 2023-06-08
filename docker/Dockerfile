FROM golang:1.19-alpine

RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh
WORKDIR /app
COPY ../go.mod go.sum ./
COPY ../.env ./
RUN go mod download
RUN go build -o proxy cmd/chat/main.go
RUN cp cmd/proxy/.env .env
EXPOSE 8080
CMD ["./main"]