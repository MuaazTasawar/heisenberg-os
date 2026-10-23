"""Shared-secret authentication between the Go API and the ML service.

The key is read from the ML_API_KEY environment variable on every request, so it can be
rotated by restarting the process and is easy to override in tests. When the variable is
empty the check is disabled (local development only).
"""
import os
import secrets

from fastapi import Header, HTTPException, status


def require_api_key(x_api_key: str | None = Header(default=None)) -> None:
    expected = os.environ.get("ML_API_KEY", "")
    if not expected:
        return
    supplied = (x_api_key or "").encode("utf-8")
    if not secrets.compare_digest(supplied, expected.encode("utf-8")):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="invalid or missing API key",
        )
