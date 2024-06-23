# Start from the official Golang base image
FROM golang:1.22.4-alpine as builder

# Set the working directory inside the container
WORKDIR /app

# Copy everything in the current directory into the container
COPY . .

# Build the Go application
RUN go build -o bookma ./cmd/bookma.go

# Start from the official Alpine image
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the executable from the builder container to the /app directory in the new container

COPY --from=builder /app/bookma /app/bookma

# Expose the port the application runs on
EXPOSE 8080

# Set environment variables (if needed, these can also be set at runtime)
# ENV AIMHARDER_HOST=your_host
# ENV AIMHARDER_BOX_ID=your_box_id
# ENV AIMHARDER_COOKIE_NAME=your_cookie_name
# ENV AIMHARDER_COOKIE_VALUE=your_cookie_value

# Command to run the executable
CMD ["./bookma"]
