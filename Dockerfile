FROM golang:1.23.4-alpine AS builder

# Set environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Create and set the working directory
WORKDIR /app

# Copy go.mod and go.sum first (to leverage Docker caching)
COPY . .

# Download and cache dependencies
RUN go mod tidy

# Copy the rest of the application source code

# Build the application binary
RUN go build -o my-app main.go



# Use the official Elasticsearch image as the base
FROM docker.elastic.co/elasticsearch/elasticsearch:8.10.2

# Copy custom elasticsearch.yml into the container
COPY elasticsearch.yml /usr/share/elasticsearch/config/elasticsearch.yml

COPY --from=builder /app/my-app .

# Set environment variables
ENV discovery.type=single-node
# ENV xpack.security.enabled=true
# ENV ELASTIC_PASSWORD=yourpassword

# Expose ports
EXPOSE 8080

# Set the default command to run Elasticsearch
CMD ["./my-app"]
