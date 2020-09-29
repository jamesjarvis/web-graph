# Start from the latest golang base image
FROM golang:latest as builder

LABEL maintainer="James Jarvis <git@jamesjarvis.io>"

RUN mkdir /build 
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -o crawler cmd/crawler/main.go


######## Start a new stage from scratch #######
FROM alpine

RUN adduser -S -D -H -h /app appuser
USER appuser

# EXPOSE 6060

COPY --from=builder /build/crawler /app/
WORKDIR /app
CMD ["./crawler"]