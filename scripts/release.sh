#!/bin/bash

set -e

DRY_RUN=false
FORCE=false

show_help() {
    echo "Usage: $0 [OPTIONS] <version>"
    echo "Options:"
    echo "  -h, --help     Show help"
    echo "  -n, --dry-run  Preview changes"
    echo "  -f, --force    Force tag creation"
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help) show_help; exit 0 ;;
        -n|--dry-run) DRY_RUN=true; shift ;;
        -f|--force) FORCE=true; shift ;;
        -*) echo "Unknown option $1"; exit 1 ;;
        *) VERSION="$1"; shift ;;
    esac
done

if [[ -z "${VERSION:-}" ]]; then
    echo "Version is required"
    exit 1
fi

if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
    echo "Invalid version format '$VERSION'"
    exit 1
fi

if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo "Not in a git repository"
    exit 1
fi

if [[ $(git status --porcelain) ]] && [[ "$DRY_RUN" != true ]]; then
    echo "Working directory is not clean"
    exit 1
fi

if git tag -l | grep -q "^${VERSION}$"; then
    if [[ "$FORCE" != true ]]; then
        echo "Tag '$VERSION' already exists"
        exit 1
    fi
fi

SHORT_COMMIT=$(git rev-parse --short HEAD)
echo "Version: $VERSION"
echo "Branch: $(git rev-parse --abbrev-ref HEAD)"
echo "Commit: $SHORT_COMMIT"

if [[ "$DRY_RUN" == true ]]; then
    echo "DRY RUN - Commands that would be executed:"
    if [[ "$FORCE" == true ]] && git tag -l | grep -q "^${VERSION}$"; then
        echo "  git tag -d $VERSION"
        echo "  git push origin --delete $VERSION"
    fi
    echo "  git tag -a $VERSION -m 'Release $VERSION'"
    echo "  git push origin $VERSION"
    exit 0
fi

read -p "Create release $VERSION? (y/N): " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Cancelled"
    exit 0
fi

if [[ "$FORCE" == true ]] && git tag -l | grep -q "^${VERSION}$"; then
    git tag -d "$VERSION"
    git push origin --delete "$VERSION" 2>/dev/null || true
fi

git tag -a "$VERSION" -m "Release $VERSION"
git push origin "$VERSION"

echo "Release $VERSION created successfully"
REPO=$(git config --get remote.origin.url | sed 's/.*github.com[:/]\([^.]*\).*/\1/')
echo "Monitor at: https://github.com/$REPO/actions"