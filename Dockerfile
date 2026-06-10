# GoReleaser v2 multi-arch Dockerfile
# Binary is pre-built by GoReleaser and placed in platform-specific directories
FROM alpine:3.24

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 hamcp \
    && adduser -D -u 1000 -G hamcp hamcp

# Copy pre-built binary from GoReleaser (platform-specific path)
ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/ha-mcp /usr/local/bin/ha-mcp

# Use non-root user for security
USER hamcp

# Expose MCP server port
EXPOSE 8080

ENTRYPOINT ["ha-mcp"]
CMD []
