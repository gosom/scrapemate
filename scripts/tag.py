"""Create a release tag on an up-to-date main branch."""

import re
import subprocess
import sys


def git(*args):
    result = subprocess.run(["git", *args], check=True, stdout=subprocess.PIPE, text=True)
    return result.stdout.strip()


def sync_main():
    if git("branch", "--show-current") != "main":
        sys.exit("Switch to main before creating a release tag.")
    if git("status", "--porcelain", "--untracked-files=normal"):
        sys.exit("Commit or stash your local changes before creating a release tag.")

    git("fetch", "--tags", "origin", "+refs/heads/main:refs/remotes/origin/main")
    target = git("rev-parse", "refs/remotes/origin/main")
    if git("rev-list", "--count", f"{target}..HEAD") != "0":
        sys.exit("Local main has commits missing from origin/main. Resolve them before tagging.")
    if git("rev-parse", "HEAD") != target:
        print("Fast-forwarding main to origin/main.")
        git("merge", "--ff-only", target)
    return target


def next_tag():
    versions = []
    for tag in git("tag", "--list").splitlines():
        match = re.fullmatch(r"v?([0-9]+)\.([0-9]+)\.([0-9]+)", tag)
        if match:
            versions.append(tuple(map(int, match.groups())))
    if not versions:
        return "v0.1.0"
    major, minor, patch = max(versions)
    return f"v{major}.{minor}.{patch + 1}"


def confirm(question):
    while True:
        answer = input(f"{question} [y/N] ").strip().lower()
        if answer in ("y", "yes"):
            return True
        if answer in ("", "n", "no"):
            return False
        print("Enter y or n.")


def main():
    target = sync_main()
    tag = next_tag()
    if not confirm(f"Create {tag} at {target[:12]}?"):
        tag = input("Enter a different tag, or press Enter to cancel: ").strip()
        if not tag:
            print("Cancelled.")
            return

    git("tag", "--", tag, target)
    print(f"Created local tag {tag}.")
    if confirm(f"Push {tag} to origin and trigger the release workflow?"):
        ref = f"refs/tags/{tag}"
        git("push", "--no-follow-tags", "origin", f"{ref}:{ref}")


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as error:
        sys.exit(error.returncode)
    except (EOFError, KeyboardInterrupt):
        sys.exit("\nCancelled.")
