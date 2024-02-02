# Use the official Golang image as a parent image
FROM golang:1.21.0-alpine3.18

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN go build -o paccot .
# Command to run the executable
CMD ["./paccot"]
