FROM --platform=linux/amd64 alpine:latest
ARG TARGETARCH

# install lvm tools
RUN apk add --no-cache lvm2 bash

WORKDIR /app

# copy compiled binary
COPY ./bin/storage-server-$TARGETARCH /app/server

# install grpc-health-probe
RUN wget -O /usr/local/bin/grpc-health-probe \
https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.11/grpc_health_probe-linux-amd64 && \
chmod +x /usr/local/bin/grpc-health-probe

# set permissions
RUN chmod +x /app/server

# health check
# HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
#     CMD grpc-health-probe -addr=localhost:9090 || exit 1

# create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# change ownership of the app directory
RUN chown -R appuser:appgroup /app

# switch to non-root user
USER appuser

EXPOSE 9090

ENTRYPOINT ["/app/server"]
