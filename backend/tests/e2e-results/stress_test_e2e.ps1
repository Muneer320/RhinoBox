param(
    [string]$TestDir = "C:\Users\munee\Downloads",
    [string]$ServerUrl = "http://localhost:8090"
)

$ErrorActionPreference = "Continue"
$results = @{
    StartTime = Get-Date
    TestDir = $TestDir
    ServerUrl = $ServerUrl
    Phases = @()
    Summary = @{}
}

# Phase 1: Health Check
Write-Host "`n=== PHASE 1: HEALTH CHECK ===" -ForegroundColor Cyan
$phase1Start = Get-Date
try {
    $health = Invoke-RestMethod -Uri "$ServerUrl/healthz" -TimeoutSec 5
    $phase1Result = @{
        Phase = "Health Check"
        Status = "PASSED"
        Duration = ((Get-Date) - $phase1Start).TotalSeconds
        ServerStatus = $health.status
    }
    Write-Host "✓ Server healthy" -ForegroundColor Green
} catch {
    $phase1Result = @{
        Phase = "Health Check"
        Status = "FAILED"
        Error = $_.Exception.Message
    }
    Write-Host "✗ Server health check failed" -ForegroundColor Red
    exit 1
}
$results.Phases += $phase1Result

# Scan test files
Write-Host "`n=== SCANNING TEST DATASET ===" -ForegroundColor Cyan
$allFiles = Get-ChildItem -Path $TestDir -File -Recurse -ErrorAction SilentlyContinue | 
    Where-Object { $_.Length -gt 0 -and $_.Length -lt 512MB }

Write-Host "Found $($allFiles.Count) files (under 512MB limit)"

# Phase 2: Batch Upload Tests
Write-Host "`n=== PHASE 2: BATCH MEDIA UPLOAD ===" -ForegroundColor Cyan
$phase2Start = Get-Date
$uploadResults = @()

# Group files into batches
$batches = @()
$currentBatch = @()
$currentSize = 0
$maxBatchSize = 100MB

foreach ($file in $allFiles) {
    if ($currentSize + $file.Length -gt $maxBatchSize -and $currentBatch.Count -gt 0) {
        $batches += ,@($currentBatch)
        $currentBatch = @()
        $currentSize = 0
    }
    $currentBatch += $file
    $currentSize += $file.Length
}
if ($currentBatch.Count -gt 0) {
    $batches += ,@($currentBatch)
}

Write-Host "Created $($batches.Count) batches"

$batchNum = 1
foreach ($batch in $batches) {
    $batchStart = Get-Date
    Write-Host "`nBatch $batchNum/$($batches.Count) - $($batch.Count) files, $([math]::Round(($batch | Measure-Object -Property Length -Sum).Sum/1MB, 2))MB"
    
    try {
        $form = @{}
        $fileNum = 0
        foreach ($file in $batch) {
            $form["file$fileNum"] = Get-Item $file.FullName
            $fileNum++
        }
        $form["category"] = "stress-test-batch-$batchNum"
        
        $response = Invoke-RestMethod -Uri "$ServerUrl/ingest/media" -Method Post -Form $form -TimeoutSec 120
        
        $batchDuration = ((Get-Date) - $batchStart).TotalSeconds
        $throughput = [math]::Round((($batch | Measure-Object -Property Length -Sum).Sum / 1MB) / $batchDuration, 2)
        
        $uploadResults += @{
            Batch = $batchNum
            Files = $batch.Count
            SizeMB = [math]::Round(($batch | Measure-Object -Property Length -Sum).Sum/1MB, 2)
            Duration = $batchDuration
            ThroughputMBps = $throughput
            Status = "SUCCESS"
        }
        
        Write-Host "  ✓ Uploaded in $([math]::Round($batchDuration, 2))s ($throughput MB/s)" -ForegroundColor Green
    } catch {
        $uploadResults += @{
            Batch = $batchNum
            Files = $batch.Count
            Status = "FAILED"
            Error = $_.Exception.Message
        }
        Write-Host "  ✗ Failed: $($_.Exception.Message)" -ForegroundColor Red
    }
    $batchNum++
}

$phase2Duration = ((Get-Date) - $phase2Start).TotalSeconds
$phase2Result = @{
    Phase = "Batch Media Upload"
    Duration = $phase2Duration
    Batches = $uploadResults
    TotalFiles = ($uploadResults | Where-Object { $_.Status -eq "SUCCESS" } | Measure-Object -Property Files -Sum).Sum
    SuccessRate = [math]::Round((($uploadResults | Where-Object { $_.Status -eq "SUCCESS" }).Count / $uploadResults.Count) * 100, 2)
}
$results.Phases += $phase2Result

Write-Host "`n=== PHASE 3: SEARCH TESTS ===" -ForegroundColor Cyan
$phase3Start = Get-Date
$searchTests = @()

# Test 1: Search by name
Write-Host "`nTest 3.1: Search by filename"
try {
    $searchStart = Get-Date
    $searchResult = Invoke-RestMethod -Uri "$ServerUrl/files/search?name=test" -TimeoutSec 10
    $searchTests += @{
        Test = "Search by name"
        Duration = ((Get-Date) - $searchStart).TotalMilliseconds
        Results = $searchResult.count
        Status = "SUCCESS"
    }
    Write-Host "  ✓ Found $($searchResult.count) files in $([math]::Round(((Get-Date) - $searchStart).TotalMilliseconds, 2))ms" -ForegroundColor Green
} catch {
    $searchTests += @{ Test = "Search by name"; Status = "FAILED"; Error = $_.Exception.Message }
    Write-Host "  ✗ Failed" -ForegroundColor Red
}

# Test 2: Search by extension
Write-Host "`nTest 3.2: Search by extension"
try {
    $searchStart = Get-Date
    $searchResult = Invoke-RestMethod -Uri "$ServerUrl/files/search?extension=jpg" -TimeoutSec 10
    $searchTests += @{
        Test = "Search by extension"
        Duration = ((Get-Date) - $searchStart).TotalMilliseconds
        Results = $searchResult.count
        Status = "SUCCESS"
    }
    Write-Host "  ✓ Found $($searchResult.count) JPG files in $([math]::Round(((Get-Date) - $searchStart).TotalMilliseconds, 2))ms" -ForegroundColor Green
} catch {
    $searchTests += @{ Test = "Search by extension"; Status = "FAILED"; Error = $_.Exception.Message }
    Write-Host "  ✗ Failed" -ForegroundColor Red
}

# Test 3: Search by type
Write-Host "`nTest 3.3: Search by type"
try {
    $searchStart = Get-Date
    $searchResult = Invoke-RestMethod -Uri "$ServerUrl/files/search?type=image" -TimeoutSec 10
    $searchTests += @{
        Test = "Search by type"
        Duration = ((Get-Date) - $searchStart).TotalMilliseconds
        Results = $searchResult.count
        Status = "SUCCESS"
    }
    Write-Host "  ✓ Found $($searchResult.count) images in $([math]::Round(((Get-Date) - $searchStart).TotalMilliseconds, 2))ms" -ForegroundColor Green
} catch {
    $searchTests += @{ Test = "Search by type"; Status = "FAILED"; Error = $_.Exception.Message }
    Write-Host "  ✗ Failed" -ForegroundColor Red
}

$phase3Result = @{
    Phase = "Search Tests"
    Duration = ((Get-Date) - $phase3Start).TotalSeconds
    Tests = $searchTests
    SuccessRate = [math]::Round((($searchTests | Where-Object { $_.Status -eq "SUCCESS" }).Count / $searchTests.Count) * 100, 2)
}
$results.Phases += $phase3Result

# Save results
$results.EndTime = Get-Date
$results.TotalDuration = ($results.EndTime - $results.StartTime).TotalSeconds
$results.Summary = @{
    TotalFiles = $phase2Result.TotalFiles
    UploadSuccessRate = $phase2Result.SuccessRate
    SearchSuccessRate = $phase3Result.SuccessRate
    TotalDuration = $results.TotalDuration
}

$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$outputFile = "stress_test_results_$timestamp.json"
$results | ConvertTo-Json -Depth 10 | Out-File $outputFile

Write-Host "`n=== TEST COMPLETE ===" -ForegroundColor Green
Write-Host "Results saved to: $outputFile"
return $results
