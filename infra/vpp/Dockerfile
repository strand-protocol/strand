##############################################################################
# VPP Container Image for Strand Protocol
#
# Base: FD.io official VPP 24.06 release image.
# Mode: container-mode (no PCI / DPDK), uses TAP virtual interfaces.
#
# Build context: ./infra/vpp
##############################################################################

FROM fdiorelease/vpp:24.06

# Install iproute2 for `ip link` / `ip addr` commands used in setup.sh
RUN apt-get update && apt-get install -y iproute2 && rm -rf /var/lib/apt/lists/*

# Create directories VPP expects at runtime
RUN mkdir -p /var/log/vpp /run/vpp

# Copy VPP startup configuration
COPY startup.conf /etc/vpp/startup.conf

# Copy setup and healthcheck scripts
COPY setup.sh /setup.sh
RUN chmod +x /setup.sh

COPY healthcheck.sh /healthcheck.sh
RUN chmod +x /healthcheck.sh

# Expose VPP stats socket and CLI socket via volumes (see docker-compose.network.yml)
VOLUME ["/run/vpp"]

# Health check: succeeds once VPP responds to "show version"
HEALTHCHECK --interval=5s --timeout=3s --retries=5 CMD /healthcheck.sh

# Entrypoint:
#   1. Start VPP in background
#   2. Wait 3 seconds for the daemon to initialize its CLI socket
#   3. Run setup.sh to configure TAP interfaces, bridge domain, and routes
#   4. Wait for the VPP background process (keeps the container alive)
CMD ["bash", "-c", "vpp -c /etc/vpp/startup.conf & sleep 3 && /setup.sh && wait"]
