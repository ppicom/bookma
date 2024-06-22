# Start from the official Golang base image
FROM golang:1.22.4-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files first
COPY go.mod go.sum ./

# Download the Go module dependencies
RUN go mod download

# Copy the source code into the container
COPY cmd/bookma.go .

# Build the Go application
RUN go build -o bookma bookma.go

# Set environment variables (if needed, these can also be set at runtime)
# ENV AIMHARDER_HOST=your_host
# ENV AIMHARDER_BOX_ID=your_box_id
# ENV AIMHARDER_COOKIE_NAME=your_cookie_name
# ENV AIMHARDER_COOKIE_VALUE=your_cookie_value

# Command to run the executable
CMD ["./bookma"]