"""A PostgreSQL for the end-to-end suite, the way the Go tests get one.

Given ``TEST_DATABASE_URL`` (CI's service container), a fresh database is
created on that server. Without it, a throwaway cluster is started from the
PostgreSQL 16 binaries, exactly as ``internal/platform/dbtest`` does, and
stopped at the end of the session.
"""

from __future__ import annotations

import os
import secrets
import shutil
import socket
import subprocess
import tempfile
import time
from dataclasses import dataclass
from urllib.parse import urlsplit, urlunsplit

import psycopg

BIN = "/usr/lib/postgresql/16/bin"
APP_ROLE = "marketplace_app"


@dataclass
class Database:
    name: str
    owner_url: str
    app_url: str


def _free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def _as_postgres(*command: str) -> None:
    subprocess.run(["su", "postgres", "-c", " ".join(f"'{c}'" for c in command)],
                   check=True, capture_output=True, text=True)


def _start_cluster():
    data = tempfile.mkdtemp(prefix="marketplace-e2e-", dir="/var/tmp")
    shutil.chown(data, "postgres", "postgres")
    os.chmod(data, 0o750)
    _as_postgres(f"{BIN}/initdb", "-D", data, "-U", "postgres", "--auth=trust", "--no-sync")
    port = _free_port()
    _as_postgres(f"{BIN}/pg_ctl", "-D", data, "-o", f"-p {port} -h 127.0.0.1 -k {data} -F",
                 "-l", f"{data}/server.log", "-w", "start")

    def stop() -> None:
        _as_postgres(f"{BIN}/pg_ctl", "-D", data, "-m", "immediate", "-w", "stop")
        shutil.rmtree(data, ignore_errors=True)

    return f"postgres://postgres@127.0.0.1:{port}/postgres?sslmode=disable", stop


def _with_database(server: str, name: str, user: str | None = None, password: str | None = None) -> str:
    parts = urlsplit(server)
    netloc = parts.netloc
    if user is not None:
        host = netloc.split("@")[-1]
        netloc = f"{user}:{password}@{host}"
    return urlunsplit((parts.scheme, netloc, "/" + name, parts.query, ""))


def provision():
    """Return a Database and the function that removes it."""
    server = os.environ.get("TEST_DATABASE_URL", "").strip()
    stop_cluster = None
    if not server:
        server, stop_cluster = _start_cluster()

    name = "e2e_" + secrets.token_hex(6)
    password = secrets.token_hex(16)
    deadline = time.monotonic() + 30
    while True:
        try:
            with psycopg.connect(server, autocommit=True) as conn:
                conn.execute(f'CREATE DATABASE "{name}"')
                conn.execute(
                    f"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{APP_ROLE}') "
                    f"THEN CREATE ROLE {APP_ROLE} LOGIN; END IF; END $$")
                conn.execute(f"ALTER ROLE {APP_ROLE} PASSWORD '{password}'")
            break
        except psycopg.OperationalError:
            if time.monotonic() > deadline:
                raise
            time.sleep(0.5)

    database = Database(
        name=name,
        owner_url=_with_database(server, name),
        app_url=_with_database(server, name, APP_ROLE, password),
    )

    def remove() -> None:
        with psycopg.connect(server, autocommit=True) as conn:
            conn.execute(f'DROP DATABASE IF EXISTS "{name}" WITH (FORCE)')
        if stop_cluster is not None:
            stop_cluster()

    return database, remove
