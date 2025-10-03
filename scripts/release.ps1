# PowerShell Release Script for Message Dispatcher
# This script helps create tagged releases that trigger the GitHub Actions workflow

param(
    [Parameter(Mandatory=$true, Position=0)]
    [string]$Version,
    
    [switch]$DryRun,
    [switch]$Force,
    [switch]$Help
)

# Show help information
function Show-Help {
    Write-Host "Release Script for Message Dispatcher" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage: .\release.ps1 [OPTIONS] <version>" -ForegroundColor White
    Write-Host ""
    Write-Host "Creates a new release by tagging the current commit and pushing to origin." -ForegroundColor Gray
    Write-Host "This triggers the GitHub Actions workflow to build and release binaries." -ForegroundColor Gray
    Write-Host ""
    Write-Host "Arguments:" -ForegroundColor White
    Write-Host "  version     Version to release (e.g., v1.0.0, v1.2.3-beta.1)" -ForegroundColor Gray
    Write-Host ""
    Write-Host "Options:" -ForegroundColor White
    Write-Host "  -Help       Show this help message" -ForegroundColor Gray
    Write-Host "  -DryRun     Show what would be done without making changes" -ForegroundColor Gray
    Write-Host "  -Force      Force tag creation even if tag exists" -ForegroundColor Gray
    Write-Host ""
    Write-Host "Examples:" -ForegroundColor White
    Write-Host "  .\release.ps1 v1.0.0                    # Create release v1.0.0" -ForegroundColor Gray
    Write-Host "  .\release.ps1 v1.2.3-beta.1            # Create pre-release v1.2.3-beta.1" -ForegroundColor Gray
    Write-Host "  .\release.ps1 -DryRun v1.0.0            # Preview release v1.0.0" -ForegroundColor Gray
    Write-Host "  .\release.ps1 -Force v1.0.0             # Force create v1.0.0 even if exists" -ForegroundColor Gray
    Write-Host ""
    Write-Host "Version Format:" -ForegroundColor White
    Write-Host "  - Must start with 'v' (e.g., v1.0.0)" -ForegroundColor Gray
    Write-Host "  - Follow semantic versioning (MAJOR.MINOR.PATCH)" -ForegroundColor Gray
    Write-Host "  - Pre-releases can include suffixes (e.g., -beta.1, -rc.1)" -ForegroundColor Gray
}

# Show help if requested
if ($Help) {
    Show-Help
    exit 0
}

# Validate version format
if ($Version -notmatch '^v\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$') {
    Write-Host "Error: Invalid version format '$Version'" -ForegroundColor Red
    Write-Host "Version must follow semantic versioning format (e.g., v1.0.0, v1.2.3-beta.1)" -ForegroundColor Gray
    exit 1
}

# Check if we're in a git repository
try {
    $null = git rev-parse --git-dir 2>$null
} catch {
    Write-Host "Error: Not in a git repository" -ForegroundColor Red
    exit 1
}

# Check if working directory is clean
if (-not $DryRun) {
    $status = git status --porcelain
    if ($status) {
        Write-Host "Error: Working directory is not clean" -ForegroundColor Red
        Write-Host "Please commit or stash your changes before creating a release." -ForegroundColor Gray
        git status --short
        exit 1
    }
}

# Check if tag already exists
$existingTag = git tag -l $Version
if ($existingTag -and -not $Force) {
    Write-Host "Error: Tag '$Version' already exists" -ForegroundColor Red
    Write-Host "Use -Force to overwrite existing tag or choose a different version." -ForegroundColor Gray
    exit 1
} elseif ($existingTag -and $Force) {
    Write-Host "Warning: Tag '$Version' already exists and will be overwritten" -ForegroundColor Yellow
}

# Get current branch and commit information
$currentBranch = git rev-parse --abbrev-ref HEAD
$currentCommit = git rev-parse HEAD
$shortCommit = git rev-parse --short HEAD

# Show release information
Write-Host "Message Dispatcher Release" -ForegroundColor Cyan
Write-Host "=========================" -ForegroundColor Cyan
Write-Host "Version:      $Version" -ForegroundColor White
Write-Host "Branch:       $currentBranch" -ForegroundColor White
Write-Host "Commit:       $shortCommit ($currentCommit)" -ForegroundColor White
Write-Host "Timestamp:    $((Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ'))" -ForegroundColor White

# Determine if this is a pre-release
$preRelease = $Version -match '^v\d+\.\d+\.\d+-[a-zA-Z0-9.-]+$'
if ($preRelease) {
    Write-Host "Type:         Pre-release" -ForegroundColor Yellow
} else {
    Write-Host "Type:         Stable release" -ForegroundColor Green
}

Write-Host ""

# Show what changes will be included
Write-Host "Changes since last tag:" -ForegroundColor Yellow
try {
    $lastTag = git describe --tags --abbrev=0 2>$null
    if ($lastTag) {
        Write-Host "Since: $lastTag" -ForegroundColor Gray
        Write-Host ""
        $commits = git log --oneline --no-merges "$lastTag..HEAD" | Select-Object -First 10
        $commits | ForEach-Object { Write-Host $_ -ForegroundColor Gray }
        
        $commitCount = (git rev-list --count "$lastTag..HEAD") -as [int]
        if ($commitCount -gt 10) {
            Write-Host "... and $($commitCount - 10) more commits" -ForegroundColor Gray
        }
    } else {
        Write-Host "No previous tags found - this will be the first release" -ForegroundColor Gray
        Write-Host ""
        $commits = git log --oneline --no-merges | Select-Object -First 10
        $commits | ForEach-Object { Write-Host $_ -ForegroundColor Gray }
    }
} catch {
    Write-Host "Could not retrieve commit history" -ForegroundColor Yellow
}

Write-Host ""

# Dry run information
if ($DryRun) {
    Write-Host "DRY RUN MODE - No changes will be made" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Commands that would be executed:" -ForegroundColor White
    if ($Force -and $existingTag) {
        Write-Host "  git tag -d $Version" -ForegroundColor Gray
        Write-Host "  git push origin --delete $Version" -ForegroundColor Gray
    }
    Write-Host "  git tag -a $Version -m 'Release $Version'" -ForegroundColor Gray
    Write-Host "  git push origin $Version" -ForegroundColor Gray
    Write-Host ""
    Write-Host "This would trigger the GitHub Actions workflow to:" -ForegroundColor White
    Write-Host "  - Run tests" -ForegroundColor Gray
    Write-Host "  - Build binaries for Windows, Linux, and macOS" -ForegroundColor Gray
    Write-Host "  - Create a GitHub release with artifacts" -ForegroundColor Gray
    Write-Host "  - Build and push Docker images" -ForegroundColor Gray
    exit 0
}

# Confirmation prompt
Write-Host "Ready to create release $Version" -ForegroundColor Yellow
$confirmation = Read-Host "Continue? (y/N)"

if ($confirmation -ne 'y' -and $confirmation -ne 'Y') {
    Write-Host "Release cancelled." -ForegroundColor Gray
    exit 0
}

Write-Host ""
Write-Host "Creating release..." -ForegroundColor Cyan

try {
    # Delete existing tag if force is enabled
    if ($Force -and $existingTag) {
        Write-Host "Deleting existing local tag..." -ForegroundColor Gray
        git tag -d $Version
        
        Write-Host "Deleting existing remote tag..." -ForegroundColor Gray
        try {
            git push origin --delete $Version 2>$null
        } catch {
            # Ignore errors when deleting remote tag
        }
    }

    # Create and push the tag
    Write-Host "Creating tag $Version..." -ForegroundColor Gray
    git tag -a $Version -m "Release $Version"

    Write-Host "Pushing tag to origin..." -ForegroundColor Gray
    git push origin $Version

    Write-Host ""
    Write-Host "✓ Release $Version created successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "What happens next:" -ForegroundColor White
    Write-Host "1. GitHub Actions will automatically start building" -ForegroundColor Gray
    Write-Host "2. Tests will run across multiple environments" -ForegroundColor Gray
    Write-Host "3. Binaries will be built for Windows, Linux, and macOS" -ForegroundColor Gray
    Write-Host "4. A GitHub release will be created with artifacts" -ForegroundColor Gray
    Write-Host "5. Docker images will be built and pushed" -ForegroundColor Gray
    Write-Host ""
    
    # Try to get GitHub repository URL
    try {
        $remoteUrl = git config --get remote.origin.url
        if ($remoteUrl -match 'github\.com[:/]([^.]+)') {
            $repoPath = $matches[1]
            Write-Host "You can monitor the progress at:" -ForegroundColor White
            Write-Host "https://github.com/$repoPath/actions" -ForegroundColor Cyan
            Write-Host ""
        }
    } catch {
        # Ignore errors getting remote URL
    }
    
    Write-Host "Release process initiated! 🚀" -ForegroundColor Green

} catch {
    Write-Host "Error during release process: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}