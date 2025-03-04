# docker run -it --rm -p 8080:8080 yourplace:latest
# Start from the Golang base image at a version
FROM ubuntu:22.04

# Add maintainer metadata
LABEL maintainer="Nops <nops@yourplace.network>"

# Update the system and install Go and other dependencies
RUN apt update && apt upgrade -y && \
    apt install -y golang-go git make

# Set the current working directory inside the container
WORKDIR /app

# Copy all files
COPY . .

# Build the Go app
RUN make install clean build

RUN chmod +x /app/target/YourPlace

# Run as low privledge user
RUN useradd -m yourplace
USER yourplace

# Set the default command for Docker image so that the app runs when the container is started
CMD ["/app/target/YourPlace"]

ENTRYPOINT ["/app/target/YourPlace"]