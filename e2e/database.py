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


@dataclass
class Database:
    name: str
    role: str
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
    try:
        shutil.chown(data, "postgres", "postgres")
        os.chmod(data, 0o750)
        _as_postgres(f"{BIN}/initdb", "-D", data, "-U", "postgres", "--auth=trust", "--no-sync")
        port = _free_port()
        _as_postgres(f"{BIN}/pg_ctl", "-D", data, "-o", f"-p {port} -h 127.0.0.1 -k {data} -F",
                     "-l", f"{data}/server.log", "-w", "start")
    except Exception as exc:
        # A data directory nothing else knows about is a data directory
        # nothing else will ever remove, the same as for a cluster that did
        # start (dbtest.go's `start`, which this mirrors).
        shutil.rmtree(data, ignore_errors=True)
        detail = exc.stderr if isinstance(exc, subprocess.CalledProcessError) and exc.stderr else str(exc)
        raise RuntimeError(
            f"cannot start a PostgreSQL cluster for the suite: {detail}\n"
            "Set TEST_DATABASE_URL to use a database that is already running."
        ) from exc

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
    # A role of this run's own, not a name every run shares: on a server
    # TEST_DATABASE_URL points at, two runs at once must never rotate a
    # password the other is mid-use with, and this one is dropped in remove()
    # rather than left for the next run to find already there.
    role = "e2e_app_" + secrets.token_hex(6)
    password = secrets.token_hex(16)
    made_database = False
    try:
        deadline = time.monotonic() + 30
        while True:
            try:
                with psycopg.connect(server, autocommit=True) as conn:
                    # The role first and the database last: the database is
                    # the one statement the except below can still undo on a
                    # shared server (it has no cluster to stop), so it is the
                    # last thing that can be left half-done.
                    conn.execute(f"CREATE ROLE \"{role}\" LOGIN PASSWORD '{password}'")
                    conn.execute(f'CREATE DATABASE "{name}"')
                made_database = True
                break
            except psycopg.OperationalError:
                if time.monotonic() > deadline:
                    raise
                time.sleep(0.5)
    except Exception:
        with psycopg.connect(server, autocommit=True) as conn:
            if made_database:
                conn.execute(f'DROP DATABASE IF EXISTS "{name}" WITH (FORCE)')
            conn.execute(f'DROP ROLE IF EXISTS "{role}"')
        if stop_cluster is not None:
            stop_cluster()
        raise

    database = Database(
        name=name,
        role=role,
        owner_url=_with_database(server, name),
        app_url=_with_database(server, name, role, password),
    )

    def remove() -> None:
        with psycopg.connect(server, autocommit=True) as conn:
            conn.execute(f'DROP DATABASE IF EXISTS "{name}" WITH (FORCE)')
            conn.execute(f'DROP ROLE IF EXISTS "{role}"')
        if stop_cluster is not None:
            stop_cluster()

    return database, remove
