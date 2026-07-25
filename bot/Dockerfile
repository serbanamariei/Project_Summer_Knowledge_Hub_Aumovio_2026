FROM golang:1.26-alpine
RUN apk add --no-cache git
RUN apk add --no-cache ffmpeg
WORKDIR /app
COPY comun/ ./comun/
COPY bot/ ./bot/
WORKDIR /app/bot
RUN go mod download
RUN go build -o main .
CMD ["./main"]