"""Fixtures that run the real binary the way a deployment runs it.

The suite starts the process with fake providers, waits for its start-up log
line, and talks to it over HTTP. Nothing here reaches a real external service
(see docs/requirements.md, section 25).
"""

from __future__ import annotations

import json
import os
import socket
import subprocess
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Iterator

import pytest

REPO_ROOT = Path(__file__).resolve().parent.parent
START_TIMEOUT = 30.0


def free_port() -> int:
    """Return a port nothing is listening on."""
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


@dataclass
class Server:
    """A running marketplace process."""

    base_url: str
    process: subprocess.Popen
    log_lines: list[dict] = field(default_factory=list)

    def entry(self, message: str) -> dict:
        """Return the first log entry carrying this message."""
        for entry in self.log_lines:
            if entry.get("message") == message:
                return entry
        raise AssertionError(f"no log entry {message!r} in {self.log_lines}")


@pytest.fixture(scope="session")
def binary() -> Path:
    """Build the binary once, or use the one the Makefile already built."""
    prebuilt = os.environ.get("MARKETPLACE_BINARY")
    if prebuilt:
        return Path(prebuilt)

    target = REPO_ROOT / "bin" / "marketplace"
    subprocess.run(
        ["make", "build"], cwd=REPO_ROOT, check=True, capture_output=True, text=True
    )
    return target


def start(binary: Path, env: dict[str, str]) -> subprocess.Popen:
    """Start the binary with env added to a clean environment."""
    return subprocess.Popen(
        [str(binary)],
        cwd=REPO_ROOT,
        env={"PATH": os.environ.get("PATH", ""), **env},
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
    )


@pytest.fixture
def run_server(binary: Path) -> Iterator[callable]:
    """Return a factory that starts the server with extra environment."""
    started: list[subprocess.Popen] = []

    def factory(**env: str) -> Server:
        port = free_port()
        process = start(binary, {"PORT": str(port), "PROVIDERS_MODE": "fake", **env})
        started.append(process)

        server = Server(base_url=f"http://127.0.0.1:{port}", process=process)
        deadline = time.monotonic() + START_TIMEOUT
        while time.monotonic() < deadline:
            line = process.stdout.readline()
            if not line:
                break
            server.log_lines.append(json.loads(line))
            if server.log_lines[-1].get("message") == "server started":
                return server
        raise AssertionError(
            "the server never logged its start-up line; "
            f"stderr: {process.stderr.read()}"
        )

    yield factory

    for process in started:
        process.terminate()
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:  # pragma: no cover - a hung shutdown
            process.kill()


@pytest.fixture
def server(run_server) -> Server:
    """A server running with the defaults."""
    return run_server()


@pytest.fixture(scope="session")
def screenshots() -> Path:
    """The directory CI keeps as the verification evidence."""
    directory = REPO_ROOT / "e2e" / "screenshots"
    directory.mkdir(parents=True, exist_ok=True)
    return directory
