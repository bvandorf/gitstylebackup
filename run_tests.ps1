# Run Go tests with verbose output and save to test_output.txt
Write-Host "Running tests..."
go test -v ./tests *> .\test_output.txt

# Check if tests passed
$testOutput = Get-Content .\test_output.txt -Raw
if ($testOutput -match "PASS") {
    Write-Host "All tests passed!" -ForegroundColor Green
} else {
    Write-Host "Some tests failed. Check test_output.txt for details." -ForegroundColor Red
}

# Display test duration if found
if ($testOutput -match "ok\s+\S+\s+(\d+\.\d+)s") {
    $duration = $matches[1]
    Write-Host "Test duration: $duration seconds" -ForegroundColor Cyan
} 