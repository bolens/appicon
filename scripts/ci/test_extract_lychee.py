"""Known release layout is accepted; unsafe or unbounded archives write nothing."""

import importlib.util
import io
import tarfile
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "extractor", Path(__file__).with_name("extract-lychee.py")
)
extractor = importlib.util.module_from_spec(spec)
spec.loader.exec_module(extractor)


class LycheeArchiveTests(unittest.TestCase):
    def make_archive(self, path, extras=()):
        with tarfile.open(path, "w:gz") as archive:
            root = tarfile.TarInfo(extractor.ROOT)
            root.type = tarfile.DIRTYPE
            archive.addfile(root)
            binary = tarfile.TarInfo(extractor.BINARY)
            payload = b"known executable bytes\n"
            binary.size = len(payload)
            archive.addfile(binary, io.BytesIO(payload))
            for member in extras:
                data = io.BytesIO(b"\0" * member.size) if member.isfile() and member.size else None
                archive.addfile(member, data)

    def test_extracts_only_known_nested_binary(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.make_archive(root / "release.tar.gz")
            extractor.extract(root / "release.tar.gz", root / "bin")
            self.assertEqual((root / "bin/lychee").read_bytes(), b"known executable bytes\n")
            self.assertEqual((root / "bin/lychee").stat().st_mode & 0o777, 0o755)
            self.assertEqual([file.name for file in (root / "bin").iterdir()], ["lychee"])

    def test_rejects_unsafe_members_before_output(self):
        cases = []
        for name in ["/escape", extractor.ROOT + "/../escape", "unexpected/lychee", extractor.BINARY]:
            cases.append([tarfile.TarInfo(name)])
        for kind in [tarfile.SYMTYPE, tarfile.LNKTYPE, tarfile.FIFOTYPE]:
            member = tarfile.TarInfo(extractor.ROOT + "/unsafe")
            member.type, member.linkname = kind, "target"
            cases.append([member])
        cases.append([tarfile.TarInfo(extractor.ROOT + f"/extra{number}")
                      for number in range(extractor.MAX_MEMBERS)])
        oversized = tarfile.TarInfo(extractor.ROOT + "/oversized")
        oversized.size = extractor.MAX_BYTES + 1
        cases.append([oversized])
        for members in cases:
            with self.subTest(member=members[0].name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                self.make_archive(root / "release.tar.gz", members)
                with self.assertRaises(ValueError):
                    extractor.extract(root / "release.tar.gz", root / "bin")
                self.assertFalse((root / "bin").exists())


if __name__ == "__main__":
    unittest.main()
