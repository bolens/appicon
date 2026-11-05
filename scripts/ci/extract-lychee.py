#!/usr/bin/env python3
"""Extract only the verified release archive's known Lychee executable."""

import argparse
import shutil
import tarfile
from pathlib import Path, PurePosixPath

ROOT = "lychee-x86_64-unknown-linux-gnu"
BINARY = ROOT + "/lychee"
MAX_MEMBERS = 32
MAX_BYTES = 32 * 1024 * 1024


def extract(archive_path, destination):
    if archive_path.stat().st_size > MAX_BYTES:
        raise ValueError("Lychee archive exceeds the release size limit")
    with tarfile.open(archive_path, mode="r:gz") as archive:
        names, binary, total = set(), None, 0
        for member in archive:
            path = PurePosixPath(member.name)
            if (path.is_absolute() or ".." in path.parts or not path.parts
                    or path.parts[0] != ROOT or path.as_posix() in names
                    or not (member.isdir() or member.isfile())):
                raise ValueError("unsafe Lychee release archive member")
            names.add(path.as_posix())
            total += member.size
            if len(names) > MAX_MEMBERS or member.size < 0 or total > MAX_BYTES:
                raise ValueError("Lychee archive exceeds the release bounds")
            if member.name == BINARY:
                if not member.isfile() or member.size == 0:
                    raise ValueError("Lychee executable must be a nonempty regular file")
                binary = member
        if binary is None:
            raise ValueError("known Lychee release executable is missing")
        destination.mkdir(parents=True, exist_ok=True)
        output = destination / "lychee"
        with archive.extractfile(binary) as source, output.open("xb") as target:
            shutil.copyfileobj(source, target)
        output.chmod(0o755)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("archive", type=Path)
    parser.add_argument("destination", type=Path)
    args = parser.parse_args()
    extract(args.archive, args.destination)


if __name__ == "__main__":
    main()
