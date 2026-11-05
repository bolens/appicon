"""The validation-only profile is exact, secret-free and checked without mutation."""

import importlib.util
import tempfile
import unittest
from pathlib import Path

SPEC = importlib.util.spec_from_file_location(
    "validation_profile", Path(__file__).with_name("sync-woodpecker-validation.py")
)
profile = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(profile)
PREFIX = b"steps:\n- name: verify\n  image: alpine\n  commands: [true]\n"
PUBLISHERS = (
    b"- name: release-publish\n  environment: {TOKEN: {from_secret: github_publish_lease}}\n"
    b"- name: pages-publish\n  commands: [true]\n"
)
SOURCE = PREFIX + profile.MARKER + PUBLISHERS


class ValidationProfileTests(unittest.TestCase):
    def test_extracts_the_exact_canonical_prefix(self):
        self.assertEqual(profile.validation_bytes(SOURCE), PREFIX)
        self.assertNotIn(b"from_secret", profile.validation_bytes(SOURCE))

    def test_invalid_boundary_or_secret_reference_cannot_write_a_profile(self):
        cases = [
            SOURCE.replace(profile.MARKER, b""), SOURCE + profile.MARKER,
            PREFIX + profile.MARKER + PUBLISHERS.replace(b"release-publish", b"other-step"),
            SOURCE + b"- name: unexpected-validation\n",
            b"environment: {TOKEN: {from_secret: private}}\n" + SOURCE,
        ]
        for source in cases:
            with self.subTest(source=source), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / ".woodpecker.yml").write_bytes(source)
                with self.assertRaises(ValueError):
                    profile.sync(root, write=True)
                self.assertFalse((root / ".woodpecker-validation").exists())

    def test_check_is_read_only_and_write_repairs_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / ".woodpecker.yml").write_bytes(SOURCE)
            target = root / ".woodpecker-validation/woodpecker.yml"
            with self.assertRaises(ValueError):
                profile.sync(root)
            self.assertFalse(target.exists())
            profile.sync(root, write=True)
            profile.sync(root)
            target.write_bytes(b"manual drift\n")
            with self.assertRaises(ValueError):
                profile.sync(root)
            self.assertEqual(target.read_bytes(), b"manual drift\n")
            profile.sync(root, write=True)
            self.assertEqual(target.read_bytes(), PREFIX)

    def test_unexpected_profile_files_are_rejected_without_deletion(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / ".woodpecker.yml").write_bytes(SOURCE)
            profile.sync(root, write=True)
            extra = root / ".woodpecker-validation/extra.yml"
            extra.write_bytes(b"caller data\n")
            for write in (False, True):
                with self.subTest(write=write), self.assertRaises(ValueError):
                    profile.sync(root, write=write)
            self.assertEqual(extra.read_bytes(), b"caller data\n")

    def test_symlink_target_is_rejected_without_touching_the_destination(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / ".woodpecker.yml").write_bytes(SOURCE)
            target = root / ".woodpecker-validation/woodpecker.yml"
            target.parent.mkdir()
            outside = root / "preserve"
            outside.write_bytes(b"caller data\n")
            target.symlink_to(outside)
            with self.assertRaises(ValueError):
                profile.sync(root, write=True)
            self.assertEqual(outside.read_bytes(), b"caller data\n")


if __name__ == "__main__":
    unittest.main()
