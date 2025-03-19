# syntax=docker/dockerfile:1

#######################
### Building container

FROM golang:latest AS build
WORKDIR /app

# Install dependencies
COPY go.mod go.sum .
RUN go mod download

# Copy source
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o output/application cmd/*.go

######################
### Running container

FROM alpine:latest AS run
WORKDIR /app

# Copy the application executable from the build image
COPY --from=build /app/output /app

# Expose the website on port 80
EXPOSE 80

# Have a little runner script that copies the default config and plugins to the
# host directory if not yet present
COPY ./config /app/default-config
COPY crontab.txt /etc/crontabs/root
COPY Makefile /app/Makefile
RUN cat >./run-application.sh <<EOF
#!/bin/sh
if [ ! -f "/app/config/websites.txt" ]; then
    echo "Copying default config"
    mkdir -p /app/config
    cp -R /app/default-config/* /app/config/
fi
make fetch &
crond
SERVER_PORT=80 ./application
EOF
RUN chmod +x ./run-application.sh
RUN apk add --no-cache make
RUN apk add --no-cache wget

# Run the application
CMD ["./run-application.sh"]
