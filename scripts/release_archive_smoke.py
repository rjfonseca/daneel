"""Download a native draft-release archive and smoke-test its installation."""
import argparse
import hashlib
import os
from pathlib import Path
import platform
import subprocess
import sys
import tempfile


def verify_release(tag, repo):
    systems = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}
    architectures = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}
    try:
        goos = systems[platform.system()]
        goarch = architectures[platform.machine().lower()]
    except KeyError as error:
        raise ValueError("Unsupported native release platform") from error
    version = tag.removeprefix("v")
    extension = "zip" if goos == "windows" else "tar.gz"
    archive_name = f"daneel_{version}_{goos}_{goarch}.{extension}"
    with tempfile.TemporaryDirectory(prefix="daneel-release-") as directory:
        root = Path(directory)
        subprocess.run(["gh", "release", "download", tag, "--repo", repo,
                        "--pattern", archive_name, "--pattern", "checksums.txt",
                        "--dir", directory], check=True, timeout=300)
        archive = root / archive_name
        entries = []
        for line in (root / "checksums.txt").read_text(encoding="utf-8").splitlines():
            fields = line.split(maxsplit=1)
            if len(fields) == 2 and fields[1].removeprefix("*") == archive_name:
                entries.append(fields[0])
        if len(entries) != 1:
            raise ValueError(f"Expected exactly one checksum entry for {archive_name}")
        with archive.open("rb") as package:
            actual = hashlib.file_digest(package, "sha256").hexdigest()
        if entries[0].lower() != actual:
            raise ValueError(f"SHA256 mismatch for {archive_name}")
        subprocess.run([sys.executable, str(Path(__file__).with_name("smoke.py")),
                        "--archive", str(archive)], check=True, timeout=600)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", default=os.environ.get("RELEASE_TAG"))
    parser.add_argument("--repo", default=os.environ.get("GITHUB_REPOSITORY"))
    args = parser.parse_args()
    if not args.tag or not args.repo:
        parser.error("provide --tag/RELEASE_TAG and --repo/GITHUB_REPOSITORY")
    verify_release(args.tag, args.repo)


if __name__ == "__main__":
    main()
