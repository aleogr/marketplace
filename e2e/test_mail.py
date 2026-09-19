"""The platform's e-mail, read where the fake provider leaves it.

The adapter selected by ``PROVIDERS_MODE=fake`` writes each message to a
directory instead of sending it, which is what makes a message something this
suite can open and read: subject, text part and HTML part, exactly as a reader
would have received them (docs/requirements.md, section 25).

The binary sends one by hand with ``send-probe``, which is also how a sending
domain is proved to work in a deployment once its records are published
(docs/roadmap.md, F11).
"""

from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parent.parent


def send_probe(binary: Path, directory: Path, address: str, language: str):
    """Run the binary's delivery probe against the fake provider."""
    return subprocess.run(
        [str(binary), "send-probe", address, language],
        cwd=REPO_ROOT,
        env={
            "PATH": os.environ.get("PATH", ""),
            "PROVIDERS_MODE": "fake",
            "MAIL_DIRECTORY": str(directory),
        },
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )


def mailbox(directory: Path) -> list[dict]:
    """Every message the fake provider has written."""
    return [json.loads(path.read_text()) for path in sorted(directory.glob("*.json"))]


@pytest.mark.local_process
def test_the_fake_provider_writes_the_message_where_it_can_be_read(binary, tmp_path):
    directory = tmp_path / "mailbox"

    result = send_probe(binary, directory, "Reader@Example.Test", "pt-BR")
    assert result.returncode == 0, result.stderr

    messages = mailbox(directory)
    assert len(messages) == 1, f"the mailbox holds {len(messages)} messages"

    message = messages[0]
    # Lower-cased on the way in, which is the form the suppression list
    # compares against.
    assert message["to"] == "reader@example.test"
    assert message["subject"], "the message was written without a subject"
    assert message["text"].strip(), "the message has no text part"
    assert "<p>" in message["html"], "the message has no HTML part"
    assert message["language"] == "pt-BR"


@pytest.mark.local_process
def test_a_message_is_written_in_the_language_it_was_asked_for(binary, tmp_path):
    """The definition of done: every user-facing text in both languages."""
    subjects = {}
    for language in ("en-US", "pt-BR"):
        directory = tmp_path / language

        result = send_probe(binary, directory, "reader@example.test", language)
        assert result.returncode == 0, result.stderr

        messages = mailbox(directory)
        assert len(messages) == 1
        subjects[language] = messages[0]["subject"]

    assert subjects["en-US"] != subjects["pt-BR"], (
        f"both languages produced the same subject {subjects['en-US']!r}, "
        "so one of them is not translated"
    )


@pytest.mark.local_process
def test_a_language_the_platform_does_not_speak_is_refused(binary, tmp_path):
    result = send_probe(binary, tmp_path / "mailbox", "reader@example.test", "fr-FR")

    assert result.returncode != 0, "the probe accepted a language with no catalogue"
    assert not list((tmp_path / "mailbox").glob("*.json"))
