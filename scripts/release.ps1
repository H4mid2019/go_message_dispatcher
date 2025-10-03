param(
    [Parameter(Mandatory=$true, Position=0)]
    [string]$Version,
    
    [switch]$DryRun,
    [switch]$Force,
    [switch]$Help
)

function Show-Help {
    Write-Output "Usage: .\release.ps1 [OPTIONS] <version>"
    Write-Output ""
    Write-Output "Arguments:"
    Write-Output "  version     Version to release (e.g., v1.0.0, v1.2.3-beta.1)"
    Write-Output ""
    Write-Output "Options:"
    Write-Output "  -Help       Show this help"
    Write-Output "  -DryRun     Preview changes without making them"
    Write-Output "  -Force      Force tag creation even if tag exists"
}

if ($Help) {
    Show-Help
    exit 0
}

if ($Version -notmatch '^v\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?$') {
    Write-Output "Error: Invalid version format '$Version'"
    exit 1
}

try {
    $null = git rev-parse --git-dir 2>$null
} catch {
    Write-Output "Error: Not in a git repository"
    exit 1
}

if (-not $DryRun) {
    $status = git status --porcelain
    if ($status) {
        Write-Output "Error: Working directory is not clean"
        exit 1
    }
}

$existingTag = git tag -l $Version
if ($existingTag -and -not $Force) {
    Write-Output "Error: Tag '$Version' already exists"
    exit 1
}

$currentBranch = git rev-parse --abbrev-ref HEAD
$shortCommit = git rev-parse --short HEAD

Write-Output "Version: $Version"
Write-Output "Branch: $currentBranch"
Write-Output "Commit: $shortCommit"

if ($DryRun) {
    Write-Output "DRY RUN - Commands that would be executed:"
    if ($Force -and $existingTag) {
        Write-Output "  git tag -d $Version"
        Write-Output "  git push origin --delete $Version"
    }
    Write-Output "  git tag -a $Version -m 'Release $Version'"
    Write-Output "  git push origin $Version"
    exit 0
}

$confirmation = Read-Host "Create release $Version? (y/N)"
if ($confirmation -ne 'y' -and $confirmation -ne 'Y') {
    Write-Output "Cancelled"
    exit 0
}

try {
    if ($Force -and $existingTag) {
        git tag -d $Version
        try {
            git push origin --delete $Version 2>$null
        } catch {}
    }

    git tag -a $Version -m "Release $Version"
    git push origin $Version

    Write-Output "Release $Version created successfully"
    
    try {
        $remoteUrl = git config --get remote.origin.url
        if ($remoteUrl -match 'github\.com[:/]([^.]+)') {
            $repoPath = $matches[1]
            Write-Output "Monitor at: https://github.com/$repoPath/actions"
        }
    } catch {}

} catch {
    Write-Output "Error: $($_.Exception.Message)"
    exit 1
}