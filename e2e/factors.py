"""Second factors, as a person sets them up and uses them, for the tests."""

from __future__ import annotations

import base64
import hashlib
import hmac
import struct
import time


def totp(key: str, at: float | None = None) -> str:
    """The code an authenticator app set up with key shows at a moment: RFC
    6238 with SHA-1, six digits and thirty-second steps, computed here and not
    asked of the service."""
    compact = key.replace(" ", "")
    secret = base64.b32decode(compact + "=" * (-len(compact) % 8))
    counter = int((time.time() if at is None else at) // 30)
    mac = hmac.new(secret, struct.pack(">Q", counter), hashlib.sha1).digest()
    offset = mac[-1] & 0x0F
    value = struct.unpack(">I", mac[offset:offset + 4])[0] & 0x7FFFFFFF
    return f"{value % 1_000_000:06d}"
