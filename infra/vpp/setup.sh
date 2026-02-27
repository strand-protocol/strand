#!/bin/bash
##############################################################################
# VPP Post-Start Setup Script for Strand Protocol
#
# Waits for VPP to finish initializing, then:
#   1. Creates TAP interfaces tap0, tap1, tap2
#   2. Configures L2 bridge domain for StrandLink EtherType 0x9100
#   3. Assigns IP addresses
#   4. Installs routes for 172.21.0.0/24
#
# This script is idempotent: re-running it is safe because each vppctl
# command is guarded against existing-interface errors where necessary.
#
# Usage: called automatically by the container CMD, or manually:
#   docker exec strand-vpp /setup.sh
##############################################################################

set -euo pipefail

VPPCTL="${VPPCTL:-vppctl}"
MAX_WAIT_SECS=30

##############################################################################
# wait_for_vpp
# Polls VPP's CLI socket until the daemon responds to "show version".
##############################################################################
wait_for_vpp() {
    local waited=0
    echo "[setup.sh] Waiting for VPP to become ready..."
    while true; do
        if ${VPPCTL} show version > /dev/null 2>&1; then
            echo "[setup.sh] VPP is ready."
            return 0
        fi
        waited=$((waited + 1))
        if [[ ${waited} -ge ${MAX_WAIT_SECS} ]]; then
            echo "[setup.sh] ERROR: VPP did not become ready within ${MAX_WAIT_SECS}s." >&2
            exit 1
        fi
        sleep 1
    done
}

##############################################################################
# vpp
# Thin wrapper that logs the command before executing it.
##############################################################################
vpp() {
    echo "[setup.sh] vppctl $*"
    ${VPPCTL} "$@"
}

##############################################################################
# Main
##############################################################################

wait_for_vpp

echo "[setup.sh] VPP version:"
${VPPCTL} show version

# --------------------------------------------------------------------------
# Create TAP interfaces
#
# tap0 : VPP gateway interface, one end in VPP, other end (vpp0) in Linux NS.
# tap1 : Peer interface for strand-node-1 (172.21.0.1).
# tap2 : Peer interface for strand-node-2 (172.21.0.2).
#
# Each host-name becomes the Linux tap device visible in `ip link show`.
# --------------------------------------------------------------------------

echo "[setup.sh] Creating TAP interfaces..."

# Create tap0 (gateway) - suppress error if already exists
${VPPCTL} create tap id 0 host-if-name vpp0 || true
${VPPCTL} create tap id 1 host-if-name vpp1 || true
${VPPCTL} create tap id 2 host-if-name vpp2 || true

# Bring interfaces up in VPP
vpp set interface state tapcli-0 up
vpp set interface state tapcli-1 up
vpp set interface state tapcli-2 up

# --------------------------------------------------------------------------
# Assign IP addresses (Layer 3 side)
#
# tap0  = 172.21.0.254/24  (VPP gateway; /24 covers the full data-net range)
# tap1  = 172.21.0.1/32    (point-to-point address for strand-node-1)
# tap2  = 172.21.0.2/32    (point-to-point address for strand-node-2)
# --------------------------------------------------------------------------

echo "[setup.sh] Assigning IP addresses..."

vpp set interface ip address tapcli-0 172.21.0.254/24
vpp set interface ip address tapcli-1 172.21.0.1/32
vpp set interface ip address tapcli-2 172.21.0.2/32

# --------------------------------------------------------------------------
# L2 Bridge Domain for StrandLink EtherType 0x9100
#
# Bridge domain 1 groups tap1 and tap2 so that StrandLink frames arriving
# on either interface are L2-flooded/forwarded to the other.
# tap0 (gateway) is NOT added to the bridge domain so that routed traffic
# from the gateway is handled via L3, not L2 flood.
#
# bridge-domain 1:
#   - learn    : MAC learning enabled (default)
#   - forward  : L2 forwarding enabled
#   - flood    : flood to unknown MACs enabled (for initial ARP/discovery)
#   - arp-term : ARP termination on the BVI if needed
# --------------------------------------------------------------------------

echo "[setup.sh] Configuring L2 bridge domain 1 for StrandLink (EtherType 0x9100)..."

# Create bridge domain 1 with learning + forwarding + flooding
vpp set bridge-domain learn   1 enable
vpp set bridge-domain forward 1 enable
vpp set bridge-domain flood   1 enable
vpp set bridge-domain arp-term 1 enable

# Add tap1 and tap2 as bridge ports (not BVI)
vpp set interface l2 bridge tapcli-1 1
vpp set interface l2 bridge tapcli-2 1

# --------------------------------------------------------------------------
# Routes
#
# The /24 route is already covered by tap0's address assignment via
# "connected" route.  Add an explicit static route as a safety net and
# for the point-to-point /32 addresses.
# --------------------------------------------------------------------------

echo "[setup.sh] Installing routes..."

# Route to the full data subnet via VPP gateway interface
vpp ip route add 172.21.0.0/24 via tapcli-0

# Explicit host routes back to node-1 and node-2 via their tap interfaces
vpp ip route add 172.21.0.1/32 via tapcli-1
vpp ip route add 172.21.0.2/32 via tapcli-2

# --------------------------------------------------------------------------
# Bring up Linux-side TAP interfaces and assign IPs in the host namespace
#
# The Linux vpp0/vpp1/vpp2 devices appear once VPP creates the tap.
# ip link and ip addr commands configure the host side.
# --------------------------------------------------------------------------

echo "[setup.sh] Configuring Linux-side TAP devices..."

ip link set vpp0 up 2>/dev/null || true
ip link set vpp1 up 2>/dev/null || true
ip link set vpp2 up 2>/dev/null || true

# vpp0: host side sees 172.21.0.253 (avoid conflict with VPP's .254)
ip addr add 172.21.0.253/24 dev vpp0 2>/dev/null || true

# --------------------------------------------------------------------------
# Show final state
# --------------------------------------------------------------------------

echo "[setup.sh] Setup complete. Current VPP interface state:"
${VPPCTL} show interface
echo ""
echo "[setup.sh] IP addresses:"
${VPPCTL} show interface addr
echo ""
echo "[setup.sh] L2 bridge domains:"
${VPPCTL} show bridge-domain 1 detail
echo ""
echo "[setup.sh] IP FIB:"
${VPPCTL} show ip fib
