# RhinoBox End-to-End Stress Test
# Test Date: 2025-11-15
# Test Directory: C:\Users\munee\Downloads

param(
    [string]$ServerUrl = "http://localhost:8090",
    [string]$TestDir = "C:\Users\munee\Downloads",
    [string]$OutputFile = "stress_test_results.json"
)

# Initialize results object
$results = @{
    TestMetadata = @{
        TestDate = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
        ServerUrl = $ServerUrl
        TestDirectory = $TestDir
        TestStartTime = Get-Date
    }
    TestConditions = @{}
    TestDataInventory = @{}
    Operations = @{}
    Summary = @{}
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "RhinoBox End-to-End Stress Test" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# ============================================================================
# PHASE 1: ENVIRONMENT VALIDATION
# ============================================================================
Write-Host "[PHASE 1] Environment Validation" -ForegroundColor Yellow
$phaseStart = Get-Date

# Check server health
try {
    $healthCheck = Invoke-RestMethod -Uri "$ServerUrl/healthz" -Method Get -TimeoutSec 5
    Write-Host "✓ Server is healthy: $($healthCheck.status)" -ForegroundColor Green
    $results.TestConditions.ServerStatus = "Healthy"
    $results.TestConditions.ServerVersion = $healthCheck
} catch {
    Write-Host "✗ Server health check failed: $_" -ForegroundColor Red
    $results.TestConditions.ServerStatus = "Failed"
    exit 1
}

# Check test directory
if (Test-Path $TestDir) {
    Write-Host "✓ Test directory exists: $TestDir" -ForegroundColor Green
    $results.TestConditions.TestDirectoryExists = $true
} else {
    Write-Host "✗ Test directory not found: $TestDir" -ForegroundColor Red
    $results.TestConditions.TestDirectoryExists = $false
    exit 1
}

$phaseEnd = Get-Date
$results.Operations.Phase1_Validation = @{
    Duration = ($phaseEnd - $phaseStart).TotalSeconds
    Status = "Success"
}

Write-Host ""

# ============================================================================
# PHASE 2: TEST DATA INVENTORY
# ============================================================================
Write-Host "[PHASE 2] Test Data Inventory" -ForegroundColor Yellow
$phaseStart = Get-Date

Write-Host "Analyzing test directory..." -ForegroundColor Gray

# Collect all files
$allFiles = Get-ChildItem -Path $TestDir -Recurse -File -ErrorAction SilentlyContinue

# Calculate statistics
$totalFiles = $allFiles.Count
$totalSizeBytes = ($allFiles | Measure-Object -Property Length -Sum).Sum
$totalSizeMB = [math]::Round($totalSizeBytes / 1MB, 2)
$totalSizeGB = [math]::Round($totalSizeBytes / 1GB, 2)

Write-Host "  Total Files: $totalFiles" -ForegroundColor Cyan
Write-Host "  Total Size: $totalSizeMB MB ($totalSizeGB GB)" -ForegroundColor Cyan

# Group by extension
$byExtension = $allFiles | Group-Object Extension | Select-Object Name, Count, @{
    Name="SizeMB"
    Expression={[math]::Round(($_.Group | Measure-Object -Property Length -Sum).Sum / 1MB, 2)}
}

Write-Host "  File Type Distribution:" -ForegroundColor Cyan
$byExtension | Sort-Object SizeMB -Descending | ForEach-Object {
    $ext = if ($_.Name) { $_.Name } else { "(no extension)" }
    Write-Host "    $ext : $($_.Count) files, $($_.SizeMB) MB" -ForegroundColor Gray
}

# Store inventory
$results.TestDataInventory = @{
    TotalFiles = $totalFiles
    TotalSizeBytes = $totalSizeBytes
    TotalSizeMB = $totalSizeMB
    TotalSizeGB = $totalSizeGB
    FileTypeDistribution = $byExtension | ForEach-Object {
        @{
            Extension = if ($_.Name) { $_.Name } else { "(no extension)" }
            Count = $_.Count
            SizeMB = $_.SizeMB
        }
    }
}

# Expected classifications
$results.TestConditions.ExpectedCategories = @{
    ISO = @{ Count = 1; ExpectedCategory = "other" }
    EXE = @{ Count = 17; ExpectedCategory = "other" }
    MSI = @{ Count = 2; ExpectedCategory = "other" }
    TTF = @{ Count = 18; ExpectedCategory = "other" }
    WAV = @{ Count = 1; ExpectedCategory = "audio/wav" }
    PNG = @{ Count = 2; ExpectedCategory = "images/png" }
    JPG = @{ Count = 9; ExpectedCategory = "images/jpg" }
    PDF = @{ Count = 2; ExpectedCategory = "documents" }
}

$phaseEnd = Get-Date
$results.Operations.Phase2_Inventory = @{
    Duration = ($phaseEnd - $phaseStart).TotalSeconds
    Status = "Success"
    FilesAnalyzed = $totalFiles
}

Write-Host ""

# ============================================================================
# PHASE 3: BULK UPLOAD STRESS TEST (ASYNC)
# ============================================================================
Write-Host "[PHASE 3] Bulk Upload Stress Test" -ForegroundColor Yellow
$phaseStart = Get-Date

Write-Host "Uploading all files asynchronously..." -ForegroundColor Gray

# Prepare multipart form
$uploadStart = Get-Date
$boundary = [System.Guid]::NewGuid().ToString()
$contentType = "multipart/form-data; boundary=$boundary"

# Build multipart body manually for large file set
$bodyBuilder = [System.Text.StringBuilder]::new()

# Add metadata field
$null = $bodyBuilder.AppendLine("--$boundary")
$null = $bodyBuilder.AppendLine('Content-Disposition: form-data; name="metadata"')
$null = $bodyBuilder.AppendLine('Content-Type: application/json')
$null = $bodyBuilder.AppendLine()
$metadata = @{
    test = "e2e_stress_test"
    timestamp = Get-Date -Format "o"
    source = "automated_test"
} | ConvertTo-Json -Compress
$null = $bodyBuilder.AppendLine($metadata)

# For stress testing with large files, we'll upload in batches
$batchSize = 10
$batches = [Math]::Ceiling($totalFiles / $batchSize)
$uploadResults = @()

Write-Host "  Processing $batches batches of up to $batchSize files each..." -ForegroundColor Gray

for ($i = 0; $i -lt $batches; $i++) {
    $batchStart = Get-Date
    $batchFiles = $allFiles | Select-Object -Skip ($i * $batchSize) -First $batchSize
    $batchFileCount = $batchFiles.Count
    
    Write-Host "  Batch $($i + 1)/$batches : $batchFileCount files" -ForegroundColor Cyan
    
    try {
        # Create form for this batch
        $form = @{
            namespace = "stress_test_batch_$($i + 1)"
            comment = "E2E stress test batch $($i + 1) of $batches"
        }
        
        # Add files to form
        $fileArray = @()
        foreach ($file in $batchFiles) {
            $fileArray += $file
        }
        $form['files'] = $fileArray
        
        # Upload using Invoke-RestMethod
        $batchUploadStart = Get-Date
        $response = Invoke-RestMethod -Uri "$ServerUrl/ingest/async" -Method Post -Form $form -TimeoutSec 300
        $batchUploadEnd = Get-Date
        $batchDuration = ($batchUploadEnd - $batchUploadStart).TotalSeconds
        
        Write-Host "    Job ID: $($response.job_id)" -ForegroundColor Gray
        Write-Host "    Upload Time: $([math]::Round($batchDuration, 2))s" -ForegroundColor Gray
        
        $uploadResults += @{
            BatchNumber = $i + 1
            JobId = $response.job_id
            FileCount = $batchFileCount
            UploadDuration = $batchDuration
            Status = "Submitted"
        }
        
    } catch {
        Write-Host "    ✗ Batch upload failed: $_" -ForegroundColor Red
        $uploadResults += @{
            BatchNumber = $i + 1
            FileCount = $batchFileCount
            Status = "Failed"
            Error = $_.Exception.Message
        }
    }
    
    # Small delay between batches to avoid overwhelming the server
    Start-Sleep -Milliseconds 100
}

$uploadEnd = Get-Date
$uploadDuration = ($uploadEnd - $uploadStart).TotalSeconds

Write-Host "  Total Upload Time: $([math]::Round($uploadDuration, 2))s" -ForegroundColor Green
Write-Host "  Average per file: $([math]::Round($uploadDuration / $totalFiles, 3))s" -ForegroundColor Green

$results.Operations.Phase3_Upload = @{
    Duration = $uploadDuration
    TotalFiles = $totalFiles
    TotalBatches = $batches
    BatchSize = $batchSize
    AveragePerFile = $uploadDuration / $totalFiles
    BatchResults = $uploadResults
}

Write-Host ""

# ============================================================================
# PHASE 4: JOB QUEUE MONITORING
# ============================================================================
Write-Host "[PHASE 4] Job Queue Monitoring" -ForegroundColor Yellow
$phaseStart = Get-Date

Write-Host "Monitoring job queue progress..." -ForegroundColor Gray

$jobIds = $uploadResults | Where-Object { $_.JobId } | ForEach-Object { $_.JobId }
$completedJobs = 0
$failedJobs = 0
$maxWaitTime = 600 # 10 minutes
$pollInterval = 2 # seconds
$elapsedTime = 0

$jobDetails = @()

while ($completedJobs + $failedJobs -lt $jobIds.Count -and $elapsedTime -lt $maxWaitTime) {
    Start-Sleep -Seconds $pollInterval
    $elapsedTime += $pollInterval
    
    $allJobsStatus = @()
    
    foreach ($jobId in $jobIds) {
        try {
            $jobStatus = Invoke-RestMethod -Uri "$ServerUrl/jobs/$jobId" -Method Get
            $allJobsStatus += $jobStatus
            
            if ($jobStatus.status -eq "completed" -and -not ($jobDetails | Where-Object { $_.job_id -eq $jobId })) {
                $completedJobs++
                Write-Host "  ✓ Job $jobId completed (Progress: $($jobStatus.progress)%)" -ForegroundColor Green
                $jobDetails += $jobStatus
            } elseif ($jobStatus.status -eq "failed") {
                $failedJobs++
                Write-Host "  ✗ Job $jobId failed: $($jobStatus.error)" -ForegroundColor Red
                $jobDetails += $jobStatus
            }
        } catch {
            Write-Host "  ⚠ Could not fetch status for job $jobId" -ForegroundColor Yellow
        }
    }
    
    # Show progress
    $progress = [math]::Round(($completedJobs + $failedJobs) / $jobIds.Count * 100, 1)
    Write-Host "  Progress: $completedJobs completed, $failedJobs failed of $($jobIds.Count) jobs ($progress%) [${elapsedTime}s elapsed]" -ForegroundColor Cyan
}

# Get final job statistics
try {
    $queueStats = Invoke-RestMethod -Uri "$ServerUrl/jobs/stats" -Method Get
    Write-Host "  Queue Stats:" -ForegroundColor Cyan
    Write-Host "    Total Processed: $($queueStats.total_processed)" -ForegroundColor Gray
    Write-Host "    Currently Running: $($queueStats.running)" -ForegroundColor Gray
    Write-Host "    Pending: $($queueStats.pending)" -ForegroundColor Gray
    
    $results.Operations.Phase4_Monitoring = @{
        Duration = $elapsedTime
        CompletedJobs = $completedJobs
        FailedJobs = $failedJobs
        TotalJobs = $jobIds.Count
        QueueStats = $queueStats
        JobDetails = $jobDetails
    }
} catch {
    Write-Host "  ⚠ Could not fetch queue stats" -ForegroundColor Yellow
    $results.Operations.Phase4_Monitoring = @{
        Duration = $elapsedTime
        CompletedJobs = $completedJobs
        FailedJobs = $failedJobs
        TotalJobs = $jobIds.Count
        Error = $_.Exception.Message
    }
}

$phaseEnd = Get-Date
Write-Host ""

# ============================================================================
# PHASE 5: STORAGE VERIFICATION
# ============================================================================
Write-Host "[PHASE 5] Storage Verification" -ForegroundColor Yellow
$phaseStart = Get-Date

Write-Host "Verifying file storage and categorization..." -ForegroundColor Gray

$storageBasePath = "C:\Users\munee\MuneerBackup\Muneer\MainFolder\CodingPractices\Hackaton\RhinoBox\backend\data\storage"

if (Test-Path $storageBasePath) {
    # Check each category directory
    $categories = @("images", "audio", "videos", "documents", "archives", "code", "spreadsheets", "presentations", "other")
    $storedFiles = @{}
    
    foreach ($category in $categories) {
        $categoryPath = Join-Path $storageBasePath $category
        if (Test-Path $categoryPath) {
            $files = Get-ChildItem -Path $categoryPath -Recurse -File -ErrorAction SilentlyContinue
            if ($files.Count -gt 0) {
                $storedFiles[$category] = $files.Count
                Write-Host "  $category : $($files.Count) files" -ForegroundColor Cyan
            }
        }
    }
    
    $results.Operations.Phase5_Storage = @{
        Duration = ((Get-Date) - $phaseStart).TotalSeconds
        StorageBasePath = $storageBasePath
        CategorizedFiles = $storedFiles
        Status = "Success"
    }
} else {
    Write-Host "  ⚠ Storage path not found: $storageBasePath" -ForegroundColor Yellow
    $results.Operations.Phase5_Storage = @{
        Duration = ((Get-Date) - $phaseStart).TotalSeconds
        Status = "StoragePathNotFound"
    }
}

$phaseEnd = Get-Date
Write-Host ""

# ============================================================================
# PHASE 6: RETRIEVAL OPERATIONS TEST
# ============================================================================
Write-Host "[PHASE 6] Retrieval Operations Test" -ForegroundColor Yellow
$phaseStart = Get-Date

# Test search functionality
Write-Host "Testing search endpoint..." -ForegroundColor Gray
$searchTests = @()

$searchQueries = @("jpg", "png", "pdf", "wav", "exe")
foreach ($query in $searchQueries) {
    $searchStart = Get-Date
    try {
        $searchResults = Invoke-RestMethod -Uri "$ServerUrl/files/search?name=$query" -Method Get -TimeoutSec 10
        $searchDuration = ((Get-Date) - $searchStart).TotalMilliseconds
        $resultCount = if ($searchResults.files) { $searchResults.files.Count } else { 0 }
        Write-Host "  Search '$query': $resultCount results in $([math]::Round($searchDuration, 2))ms" -ForegroundColor Cyan
        
        $searchTests += @{
            Query = $query
            ResultCount = $resultCount
            Duration = $searchDuration
            Status = "Success"
        }
    } catch {
        Write-Host "  ✗ Search '$query' failed: $_" -ForegroundColor Red
        $searchTests += @{
            Query = $query
            Status = "Failed"
            Error = $_.Exception.Message
        }
    }
}

# Test metadata retrieval (if we have any file hashes)
Write-Host "Testing metadata endpoint..." -ForegroundColor Gray
$metadataTests = @()

if ($jobDetails.Count -gt 0 -and $jobDetails[0].result -and $jobDetails[0].result.files) {
    $sampleFiles = $jobDetails[0].result.files | Select-Object -First 3
    
    foreach ($file in $sampleFiles) {
        if ($file.hash) {
            $metadataStart = Get-Date
            try {
                $metadata = Invoke-RestMethod -Uri "$ServerUrl/files/metadata?hash=$($file.hash)" -Method Get -TimeoutSec 10
                $metadataDuration = ((Get-Date) - $metadataStart).TotalMilliseconds
                Write-Host "  Metadata for $($file.original_name): $([math]::Round($metadataDuration, 2))ms" -ForegroundColor Cyan
                
                $metadataTests += @{
                    FileName = $file.original_name
                    Hash = $file.hash
                    Duration = $metadataDuration
                    Status = "Success"
                }
            } catch {
                Write-Host "  ✗ Metadata retrieval failed: $_" -ForegroundColor Red
                $metadataTests += @{
                    FileName = $file.original_name
                    Status = "Failed"
                    Error = $_.Exception.Message
                }
            }
        }
    }
}

$results.Operations.Phase6_Retrieval = @{
    Duration = ((Get-Date) - $phaseStart).TotalSeconds
    SearchTests = $searchTests
    MetadataTests = $metadataTests
}

$phaseEnd = Get-Date
Write-Host ""

# ============================================================================
# SUMMARY AND REPORTING
# ============================================================================
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "TEST SUMMARY" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

$testEndTime = Get-Date
$totalDuration = ($testEndTime - $results.TestMetadata.TestStartTime).TotalSeconds

Write-Host ""
Write-Host "Test Duration: $([math]::Round($totalDuration, 2)) seconds" -ForegroundColor Green
Write-Host ""
Write-Host "Phase Breakdown:" -ForegroundColor Yellow
Write-Host "  1. Validation     : $([math]::Round($results.Operations.Phase1_Validation.Duration, 2))s" -ForegroundColor Gray
Write-Host "  2. Inventory      : $([math]::Round($results.Operations.Phase2_Inventory.Duration, 2))s" -ForegroundColor Gray
Write-Host "  3. Upload         : $([math]::Round($results.Operations.Phase3_Upload.Duration, 2))s" -ForegroundColor Gray
Write-Host "  4. Monitoring     : $([math]::Round($results.Operations.Phase4_Monitoring.Duration, 2))s" -ForegroundColor Gray
Write-Host "  5. Storage Check  : $([math]::Round($results.Operations.Phase5_Storage.Duration, 2))s" -ForegroundColor Gray
Write-Host "  6. Retrieval Test : $([math]::Round($results.Operations.Phase6_Retrieval.Duration, 2))s" -ForegroundColor Gray
Write-Host ""

# Performance metrics
$throughputMBps = $totalSizeMB / $results.Operations.Phase3_Upload.Duration
$throughputFilesPs = $totalFiles / $results.Operations.Phase3_Upload.Duration

Write-Host "Upload Performance:" -ForegroundColor Yellow
Write-Host "  Throughput: $([math]::Round($throughputMBps, 2)) MB/s" -ForegroundColor Green
Write-Host "  File Rate: $([math]::Round($throughputFilesPs, 2)) files/s" -ForegroundColor Green
Write-Host "  Average per file: $([math]::Round($results.Operations.Phase3_Upload.AveragePerFile * 1000, 2))ms" -ForegroundColor Green
Write-Host ""

Write-Host "Job Processing:" -ForegroundColor Yellow
Write-Host "  Completed: $($results.Operations.Phase4_Monitoring.CompletedJobs)/$($results.Operations.Phase4_Monitoring.TotalJobs)" -ForegroundColor Green
Write-Host "  Failed: $($results.Operations.Phase4_Monitoring.FailedJobs)" -ForegroundColor $(if ($results.Operations.Phase4_Monitoring.FailedJobs -gt 0) { "Red" } else { "Green" })
Write-Host ""

# Store summary
$results.Summary = @{
    TotalDuration = $totalDuration
    FilesProcessed = $totalFiles
    DataProcessedMB = $totalSizeMB
    UploadThroughputMBps = $throughputMBps
    UploadThroughputFilesPs = $throughputFilesPs
    JobsCompleted = $results.Operations.Phase4_Monitoring.CompletedJobs
    JobsFailed = $results.Operations.Phase4_Monitoring.FailedJobs
    OverallStatus = if ($results.Operations.Phase4_Monitoring.FailedJobs -eq 0) { "SUCCESS" } else { "PARTIAL_SUCCESS" }
}

$results.TestMetadata.TestEndTime = $testEndTime

# Save results to JSON
$resultsJson = $results | ConvertTo-Json -Depth 10
$resultsJson | Out-File -FilePath $OutputFile -Encoding UTF8
Write-Host "Results saved to: $OutputFile" -ForegroundColor Green
Write-Host ""

# Display final verdict
if ($results.Summary.OverallStatus -eq "SUCCESS") {
    Write-Host "✓ ALL TESTS PASSED" -ForegroundColor Green
} else {
    Write-Host "⚠ TESTS COMPLETED WITH ISSUES" -ForegroundColor Yellow
}
Write-Host ""
