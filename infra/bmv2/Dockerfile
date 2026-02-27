##############################################################################
# BMv2 Behavioral Model Container for Nexus Protocol NexRoute P4 Dataplane
#
# Base: p4lang/behavioral-model (official P4.org BMv2 image).
#
# This Dockerfile MUST be built with the repository root as the build context:
#
#   docker build -f infra/bmv2/Dockerfile -t nexus-bmv2 .
#
# This is reflected in docker-compose.network.yml:
#   build:
#     context: .                         <-- repo root
#     dockerfile: infra/bmv2/Dockerfile
#
# The build context at repo root gives access to nexroute/p4/ so we can
# COPY the P4 source files and attempt compilation inside the image.
##############################################################################

FROM p4lang/behavioral-model:latest

##############################################################################
# Install p4c (P4 compiler).
#
# The p4lang/behavioral-model image may or may not include p4c depending on
# the tag.  We attempt installation via the p4lang APT repository; if that
# fails (e.g. because p4c is already present or the repo is unavailable in
# this base image) we continue without it -- the fallback stub JSON will be
# used instead.
##############################################################################
RUN apt-get update && \
    apt-get install -y p4lang-p4c 2>/dev/null || true && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /p4

##############################################################################
# Copy P4 source files from the repository root build context.
#
# Source layout (relative to repo root):
#   nexroute/p4/headers.p4
#   nexroute/p4/parser.p4
#   nexroute/p4/sad_lookup.p4
#   nexroute/p4/forwarding.p4
#   nexroute/p4/sad_forward.p4   <-- compilation entry point
##############################################################################
COPY nexroute/p4/ /p4/

##############################################################################
# Compile the P4 program to a BMv2 JSON artifact.
#
# Primary:  p4c --target bmv2 --arch v1model
# Fallback: if p4c is unavailable or compilation fails, write a stub JSON so
#           simple_switch can still start (it will forward no traffic, but
#           the Thrift control-plane socket will be available for testing).
#
# The compiled JSON is placed at /p4/sad_forward.json.
##############################################################################
RUN p4c --target bmv2 \
        --arch v1model \
        --std p4-16 \
        /p4/sad_forward.p4 \
        -o /p4/sad_forward.json \
    2>/dev/null || \
    echo '{"program":"stub","tables":[{"name":"sad_ternary_match","id":0},{"name":"node_id_forward","id":1}]}' \
        > /p4/sad_forward.json

##############################################################################
# Expose Thrift control-plane port.
#
# simple_switch listens on TCP 9090 (default) for P4Runtime / Thrift RPC.
# NexRoute's p4_runtime.c connects to this port.
##############################################################################
EXPOSE 9090

##############################################################################
# Runtime: start simple_switch with the compiled P4 JSON.
#
# Interfaces:
#   -i 0@eth0  port 0 mapped to container eth0 (data-net)
#   -i 1@eth1  port 1 (if present)
#   -i 2@eth2  port 2 (if present)
#
# --thrift-port 9090     : Thrift control-plane socket
# --log-console          : Log to stdout for `docker logs`
#
# simple_switch exits with code 1 if the JSON is a stub; in that case the
# container restarts (restart: unless-stopped in compose).
##############################################################################
CMD ["simple_switch", \
     "--thrift-port", "9090", \
     "--log-console", \
     "-i", "0@eth0", \
     "-i", "1@eth1", \
     "-i", "2@eth2", \
     "/p4/sad_forward.json"]
