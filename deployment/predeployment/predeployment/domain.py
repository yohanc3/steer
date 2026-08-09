"""Production domain checks performed before requesting a TLS certificate."""

from __future__ import annotations

import socket
from collections.abc import Mapping


from .public_endpoint import PublicEndpointError


def verify_domain_points_to_deployment(environment: Mapping[str, str]) -> None:
    """Require APP_DOMAIN DNS to resolve to DEPLOYMENT_PUBLIC_IP."""
    domain = environment["APP_DOMAIN"]
    expected_ip = environment["DEPLOYMENT_PUBLIC_IP"]

    try:
        addresses = {
            result[4][0]
            for result in socket.getaddrinfo(domain, None, type=socket.SOCK_STREAM)
        }
    except socket.gaierror as error:
        raise PublicEndpointError(
            f"could not resolve {domain}: {error}; create or correct its DNS record before retrying"
        ) from error

    if expected_ip not in addresses:
        found = ", ".join(sorted(addresses)) or "no addresses"
        raise PublicEndpointError(
            f"{domain} resolves to {found}, not DEPLOYMENT_PUBLIC_IP; "
            "update DNS and wait for propagation before retrying"
        )
