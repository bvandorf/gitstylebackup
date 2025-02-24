# Build script for gitstylebackup
Write-Host "Building gitstylebackup..."

# Set environment variables
$env:GOOS = "windows"
$env:GOARCH = "amd64"

# Build the executable
Write-Host "Running go build..."
go build -o gitstylebackup.exe ./cmd/gitstylebackup

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful! Executable created: gitstylebackup.exe" -ForegroundColor Green
} else {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
} 