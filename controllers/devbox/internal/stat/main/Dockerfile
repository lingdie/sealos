FROM --platform=linux/amd64 alpine:latest

# install lvm tools
RUN apk add --no-cache lvm2 bash

WORKDIR /app

# copy compiled binary
COPY ./lvm-monitor .

# install grpc-health-probe
RUN wget -O /usr/local/bin/grpc-health-probe \
https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.11/grpc_health_probe-linux-amd64 && \
chmod +x /usr/local/bin/grpc-health-probe

# set permissions
RUN chmod +x /app/lvm-monitor

# health check
# HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
#     CMD grpc-health-probe -addr=localhost:9090 || exit 1

EXPOSE 9090

ENTRYPOINT ["/app/lvm-monitor"]
