#!/bin/bash
##############################################################################
# VPP Health Check
#
# Used by Docker HEALTHCHECK directive.  Returns 0 (healthy) if VPP responds
# to "show version", non-zero (unhealthy) otherwise.
##############################################################################
vppctl show version > /dev/null 2>&1
