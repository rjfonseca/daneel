"""Plan deterministic semantic releases without changing Git or GitHub state."""
import argparse
from dataclasses import dataclass
import json
import os
from pathlib import Path
import re
import subprocess


POLICY_PATH = ".github/release-policy.json"
TYPES = ("feat", "fix", "chore", "docs", "refactor", "perf", "test", "build", "ci", "style", "revert")
HEADER = re.compile(
    r"(?P<kind>" + "|".join(TYPES) + r")(?:\([^()\r\n]+\))?(?P<breaking>!)?: (?P<description>\S(?:[^\r\n]*\S)?)",
    re.IGNORECASE,
)
BREAKING = re.compile(r"^BREAKING(?: |-)CHANGE:[ \t]+\S", re.MULTILINE)


@dataclass(frozen=True, order=True)
class Version:
    major: int
    minor: int
    patch: int

    @classmethod
    def parse(cls, value):
        if not isinstance(value, str) or not re.fullmatch(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", value):
            raise ValueError("Expected a stable semantic version such as 1.0.0")
        return cls(*map(int, value.split(".")))

    def bump(self, kind):
        if kind == "major":
            return Version(self.major + 1, 0, 0)
        if kind == "minor":
            return Version(self.major, self.minor + 1, 0)
        if kind == "patch":
            return Version(self.major, self.minor, self.patch + 1)
        raise ValueError("Unknown semantic increment")

    def __str__(self):
        return f"{self.major}.{self.minor}.{self.patch}"


def bump_for_message(message):
    message = message.replace("\r\n", "\n").rstrip("\n")
    header, separator, body = message.partition("\n")
    match = HEADER.fullmatch(header)
    if not match or (separator and body and not body.startswith("\n")):
        raise ValueError("Expected a Conventional Commit with a supported type and nonempty description")
    if match["breaking"] or BREAKING.search(body):
        return "major"
    return "minor" if match["kind"].lower() == "feat" else "patch"


def git(root, *arguments):
    # A verifier may run inside a hook; inherited Git context must not redirect it.
    env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
    result = subprocess.run(["git", "-C", str(root), *arguments], env=env,
                            text=True, encoding="utf-8", capture_output=True, timeout=60)
    if result.returncode:
        raise ValueError("Unable to inspect release history: " + result.stderr.strip())
    return result.stdout.rstrip("\r\n")


def plan(root, ref="HEAD"):
    root = Path(root)
    sha = git(root, "rev-parse", "--verify", "--end-of-options", ref + "^{commit}")
    touches = git(root, "log", "--first-parent", "--reverse", "--format=%H", sha, "--", POLICY_PATH).splitlines()
    if not touches:
        raise ValueError("The release policy is missing from this source history")
    anchor = touches[0]
    try:
        initial = json.loads(git(root, "show", f"{anchor}:{POLICY_PATH}"))
        current = json.loads(git(root, "show", f"{sha}:{POLICY_PATH}"))
        if initial != current:
            raise ValueError("The initial version policy cannot change after its introduction")
        version = Version.parse(initial["initial_version"])
    except (KeyError, TypeError, json.JSONDecodeError) as error:
        raise ValueError("Invalid initial version policy") from error
    for commit in git(root, "rev-list", "--first-parent", "--reverse", f"{anchor}..{sha}").splitlines():
        if len(git(root, "rev-list", "--parents", "-n", "1", commit).split()) != 2:
            raise ValueError("Release history requires squash merges and a single parent per merge")
        version = version.bump(bump_for_message(git(root, "show", "-s", "--format=%B", commit)))
    # Validate the initial merge as well, even though its version is fixed.
    message = git(root, "show", "-s", "--format=%B", sha)
    bump_for_message(message)
    return {"sha": sha, "anchor": anchor, "version": str(version), "tag": "v" + str(version)}


def write_outputs(path, values):
    if path:
        with Path(path).open("a", encoding="utf-8") as output:
            for key, value in values.items():
                if not re.fullmatch(r"[a-z_]+", key) or "\n" in str(value) or "\r" in str(value):
                    raise ValueError("Unsafe workflow output")
                output.write(f"{key}={value}\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    title = commands.add_parser("validate-title")
    title.add_argument("--title", default=os.environ.get("PR_TITLE", ""))
    planning = commands.add_parser("plan")
    planning.add_argument("--repo", type=Path, default=Path.cwd())
    planning.add_argument("--ref", default="HEAD")
    planning.add_argument("--github-output", default=os.environ.get("GITHUB_OUTPUT"))
    planning.add_argument("--notes-file", type=Path)
    args = parser.parse_args()
    try:
        if args.command == "validate-title":
            if "\n" in args.title or "\r" in args.title:
                raise ValueError("PR titles must contain exactly one line")
            try:
                increment = bump_for_message(args.title)
            except ValueError as error:
                raise ValueError(
                    f"{error} (received GitHub PR title: {args.title!r}; "
                    "remove leading emojis or persona prefixes like 'Palette:' on the GitHub Pull Request title itself, "
                    "as pushing empty git commits will not change the PR title)"
                ) from error
            print("Valid Conventional Commit title; increment=" + increment)
        else:
            result = plan(args.repo, args.ref)
            write_outputs(args.github_output, result)
            if args.notes_file:
                message = git(args.repo, "show", "-s", "--format=%B", result["sha"])
                args.notes_file.write_text(f"# {result['tag']}\n\n{message}\n\nSource commit: `{result['sha']}`\n", encoding="utf-8")
            print(json.dumps(result))
    except ValueError as error:
        parser.exit(1, str(error) + "\n")


if __name__ == "__main__":
    main()
