"""Fixtures that run the real binary the way a deployment runs it.

The suite starts the process with fake providers, waits for its start-up log
line, and talks to it over HTTP. Nothing here reaches a real external service
(see docs/requirements.md, section 25).

Setting ``MARKETPLACE_BASE_URL`` points the same suite at a service that is
already running — the lab, after a deployment — instead of starting one. The
pipeline uses it to prove that what was deployed answers, rather than proving
only that what was built answers locally (docs/roadmap.md, F3).
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

LOCAL_PROCESS = "local_process"


def deployment() -> str | None:
    """The base URL of a service to test against, when one was given."""
    return os.environ.get("MARKETPLACE_BASE_URL", "").strip().rstrip("/") or None


def pytest_configure(config: pytest.Config) -> None:
    config.addinivalue_line(
        "markers",
        f"{LOCAL_PROCESS}: reads the process's own stdout or exit status, so it "
        "needs the binary in this session and cannot describe a deployment.",
    )


def pytest_collection_modifyitems(config: pytest.Config, items: list) -> None:
    """Skip what a deployment cannot answer, and only that.

    A service on the other side of the internet has no stdout to read here and
    no exit status to observe: those two facts are proved against the binary,
    on every pull request, by the same suite in its local mode. Skipping them
    against a deployment is the difference between the two targets, not a check
    being dropped — the pipeline runs both, and this run prints what it skipped.
    """
    base_url = deployment()
    if base_url is None:
        return
    skip = pytest.mark.skip(
        reason=f"reads the local process; this run targets {base_url}"
    )
    for item in items:
        if LOCAL_PROCESS in item.keywords:
            item.add_marker(skip)


def free_port() -> int:
    """Return a port nothing is listening on."""
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


@dataclass
class Server:
    """A marketplace service under test: a local process, or a deployment."""

    base_url: str
    process: subprocess.Popen | None = None
    log_lines: list[dict] = field(default_factory=list)

    def entry(self, message: str) -> dict:
        """Return the first log entry carrying this message."""
        if self.process is None:
            raise AssertionError(
                f"the log of {self.base_url} is not readable from here; "
                f"mark this test with @pytest.mark.{LOCAL_PROCESS}"
            )
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
def server(request: pytest.FixtureRequest) -> Server:
    """The service under test, running with the defaults.

    The deployment when one was named, and a process started here otherwise.
    ``run_server`` is asked for lazily so that a run against a deployment never
    builds a binary it would not use.
    """
    base_url = deployment()
    if base_url is not None:
        return Server(base_url=base_url)
    return request.getfixturevalue("run_server")()


@pytest.fixture
def site_url(server: Server) -> str:
    """An address that serves pages, as opposed to one that merely answers.

    Against a deployment, the service's own ``run.app`` address belongs to no
    marketplace, so every page there is the 404 that says exactly that
    (docs/roadmap.md, F5). The pages therefore have to be asked for at a host
    some marketplace claims, which is what ``MARKETPLACE_HOSTS`` names. A local
    process runs without a database, resolves no hosts and serves every
    address, so there its own base URL is the answer.
    """
    for host in os.environ.get("MARKETPLACE_HOSTS", "").split(","):
        if host.strip():
            return f"https://{host.strip()}"
    return server.base_url


@pytest.fixture(scope="session")
def screenshots() -> Path:
    """The directory CI keeps as the verification evidence."""
    directory = REPO_ROOT / "e2e" / "screenshots"
    directory.mkdir(parents=True, exist_ok=True)
    return directory
