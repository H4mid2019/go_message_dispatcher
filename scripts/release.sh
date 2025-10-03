#!/bin/bash

# Release script for Message Dispatcher
# This script helps create tagged releases that trigger the GitHub Actions workflow

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
DRY_RUN=false
FORCE=false

# Help function
show_help() {
    echo "Release Script for Message Dispatcher"
    echo ""
    echo "Usage: $0 [OPTIONS] <version>"
    echo ""
    echo "Creates a new release by tagging the current commit and pushing to origin."
    echo "This triggers the GitHub Actions workflow to build and release binaries."
    echo ""
    echo "Arguments:"
    echo "  version     Version to release (e.g., v1.0.0, v1.2.3-beta.1)"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -n, --dry-run  Show what would be done without making changes"
    echo "  -f, --force    Force tag creation even if tag exists"
    echo ""
    echo "Examples:"
    echo "  $0 v1.0.0                    # Create release v1.0.0"
    echo "  $0 v1.2.3-beta.1            # Create pre-release v1.2.3-beta.1"
    echo "  $0 --dry-run v1.0.0          # Preview release v1.0.0"
    echo "  $0 --force v1.0.0            # Force create v1.0.0 even if exists"
    echo ""
    echo "Version Format:"
    echo "  - Must start with 'v' (e.g., v1.0.0)"
    echo "  - Follow semantic versioning (MAJOR.MINOR.PATCH)"
    echo "  - Pre-releases can include suffixes (e.g., -beta.1, -rc.1)"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -n|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        -*)
            echo -e "${RED}Error: Unknown option $1${NC}"
            echo "Use --help for usage information."
            exit 1
            ;;
        *)
            if [[ -z "${VERSION:-}" ]]; then
                VERSION="$1"
            else
                echo -e "${RED}Error: Multiple version arguments provided${NC}"
                exit 1
            fi
            shift
            ;;
    esac
done

# Check if version is provided
if [[ -z "${VERSION:-}" ]]; then
    echo -e "${RED}Error: Version is required${NC}"
    echo "Use --help for usage information."
    exit 1
fi

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
    echo -e "${RED}Error: Invalid version format '$VERSION'${NC}"
    echo "Version must follow semantic versioning format (e.g., v1.0.0, v1.2.3-beta.1)"
    exit 1
fi

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo -e "${RED}Error: Not in a git repository${NC}"
    exit 1
fi

# Check if working directory is clean
if [[ $(git status --porcelain) ]] && [[ "$DRY_RUN" != true ]]; then
    echo -e "${RED}Error: Working directory is not clean${NC}"
    echo "Please commit or stash your changes before creating a release."
    git status --short
    exit 1
fi

# Check if tag already exists
if git tag -l | grep -q "^${VERSION}$"; then
    if [[ "$FORCE" != true ]]; then
        echo -e "${RED}Error: Tag '$VERSION' already exists${NC}"
        echo "Use --force to overwrite existing tag or choose a different version."
        exit 1
    else
        echo -e "${YELLOW}Warning: Tag '$VERSION' already exists and will be overwritten${NC}"
    fi
fi

# Get current branch and commit
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
CURRENT_COMMIT=$(git rev-parse HEAD)
SHORT_COMMIT=$(git rev-parse --short HEAD)

# Show release information
echo -e "${BLUE}Message Dispatcher Release${NC}"
echo "========================="
echo "Version:      $VERSION"
echo "Branch:       $CURRENT_BRANCH"
echo "Commit:       $SHORT_COMMIT ($CURRENT_COMMIT)"
echo "Timestamp:    $(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Determine if this is a pre-release
PRE_RELEASE=false
if [[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+-[a-zA-Z0-9.-]+$ ]]; then
    PRE_RELEASE=true
    echo "Type:         Pre-release"
else
    echo "Type:         Stable release"
fi

echo ""

# Show what changes will be included
echo -e "${YELLOW}Changes since last tag:${NC}"
LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
if [[ -n "$LAST_TAG" ]]; then
    echo "Since: $LAST_TAG"
    echo ""
    git log --oneline --no-merges ${LAST_TAG}..HEAD | head -10
    COMMIT_COUNT=$(git rev-list --count ${LAST_TAG}..HEAD)
    if [[ $COMMIT_COUNT -gt 10 ]]; then
        echo "... and $(($COMMIT_COUNT - 10)) more commits"
    fi
else
    echo "No previous tags found - this will be the first release"
    echo ""
    git log --oneline --no-merges | head -10
fi

echo ""

# Dry run information
if [[ "$DRY_RUN" == true ]]; then
    echo -e "${YELLOW}DRY RUN MODE - No changes will be made${NC}"
    echo ""
    echo "Commands that would be executed:"
    if [[ "$FORCE" == true ]] && git tag -l | grep -q "^${VERSION}$"; then
        echo "  git tag -d $VERSION"
        echo "  git push origin --delete $VERSION"
    fi
    echo "  git tag -a $VERSION -m 'Release $VERSION'"
    echo "  git push origin $VERSION"
    echo ""
    echo "This would trigger the GitHub Actions workflow to:"
    echo "  - Run tests"
    echo "  - Build binaries for Windows, Linux, and macOS"
    echo "  - Create a GitHub release with artifacts"
    echo "  - Build and push Docker images"
    exit 0
fi

# Confirmation prompt
echo -e "${YELLOW}Ready to create release $VERSION${NC}"
read -p "Continue? (y/N): " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Release cancelled."
    exit 0
fi

echo ""
echo -e "${BLUE}Creating release...${NC}"

# Delete existing tag if force is enabled
if [[ "$FORCE" == true ]] && git tag -l | grep -q "^${VERSION}$"; then
    echo "Deleting existing local tag..."
    git tag -d "$VERSION"
    
    echo "Deleting existing remote tag..."
    git push origin --delete "$VERSION" 2>/dev/null || true
fi

# Create and push the tag
echo "Creating tag $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"

echo "Pushing tag to origin..."
git push origin "$VERSION"

echo ""
echo -e "${GREEN}✓ Release $VERSION created successfully!${NC}"
echo ""
echo "What happens next:"
echo "1. GitHub Actions will automatically start building"
echo "2. Tests will run across multiple environments"
echo "3. Binaries will be built for Windows, Linux, and macOS"
echo "4. A GitHub release will be created with artifacts"
echo "5. Docker images will be built and pushed"
echo ""
echo "You can monitor the progress at:"
echo "https://github.com/$(git config --get remote.origin.url | sed 's/.*github.com[:/]\([^.]*\).*/\1/')/actions"
echo ""
echo -e "${GREEN}Release process initiated! 🚀${NC}"