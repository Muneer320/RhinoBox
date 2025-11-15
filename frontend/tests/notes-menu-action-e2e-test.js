/**
 * End-to-End Test for Notes Menu Action Feature
 * Tests the complete flow from clicking Notes button to managing notes
 * 
 * Usage:
 * 1. Start the backend server: cd backend/cmd/rhinobox && go run main.go
 * 2. Start the frontend dev server: cd frontend && npm run dev
 * 3. Open this file in a browser or run with a test runner
 */

const E2E_TEST_CONFIG = {
  API_BASE: 'http://localhost:8090',
  FRONTEND_URL: 'http://localhost:5173',
  TEST_FILE_NAME: 'e2e-test-file.jpg',
  TEST_NOTE_TEXT: 'E2E Test Note - ' + new Date().toISOString()
};

class NotesMenuActionE2ETest {
  constructor() {
    this.results = {
      passed: 0,
      failed: 0,
      total: 0,
      tests: [],
      metrics: {
        startTime: null,
        endTime: null,
        duration: null,
        apiCalls: 0,
        apiErrors: 0
      }
    };
  }

  async log(message, type = 'info') {
    const timestamp = new Date().toISOString();
    const logMessage = `[${timestamp}] ${message}`;
    console.log(logMessage);
    this.results.tests.push({ timestamp, message, type });
  }

  async assert(condition, message) {
    this.results.total++;
    if (condition) {
      this.results.passed++;
      await this.log(`✓ PASS: ${message}`, 'pass');
      return true;
    } else {
      this.results.failed++;
      await this.log(`✗ FAIL: ${message}`, 'fail');
      return false;
    }
  }

  async testBackendHealth() {
    await this.log('\n=== E2E Test 1: Backend Health Check ===');
    this.results.metrics.startTime = Date.now();
    
    try {
      this.results.metrics.apiCalls++;
      const response = await fetch(`${E2E_TEST_CONFIG.API_BASE}/healthz`);
      const isHealthy = response.ok;
      await this.assert(isHealthy, 'Backend health check passed');
      return isHealthy;
    } catch (error) {
      this.results.metrics.apiErrors++;
      await this.assert(false, `Backend health check failed: ${error.message}`);
      return false;
    }
  }

  async testUploadTestFile() {
    await this.log('\n=== E2E Test 2: Upload Test File ===');
    
    try {
      // Create a test file blob
      const testFileContent = 'E2E Test File Content';
      const testFile = new Blob([testFileContent], { type: 'image/jpeg' });
      const formData = new FormData();
      formData.append('files', testFile, E2E_TEST_CONFIG.TEST_FILE_NAME);
      
      this.results.metrics.apiCalls++;
      const response = await fetch(`${E2E_TEST_CONFIG.API_BASE}/ingest`, {
        method: 'POST',
        body: formData
      });
      
      const isSuccess = response.ok;
      await this.assert(isSuccess, 'Test file uploaded successfully');
      
      if (isSuccess) {
        const data = await response.json();
        await this.log(`Uploaded file hash: ${data.hash || 'N/A'}`, 'info');
        return data.hash || data.fileId;
      }
      
      return null;
    } catch (error) {
      this.results.metrics.apiErrors++;
      await this.assert(false, `File upload failed: ${error.message}`);
      return null;
    }
  }

  async testGetFiles() {
    await this.log('\n=== E2E Test 3: Get Files List ===');
    
    try {
      this.results.metrics.apiCalls++;
      const response = await fetch(`${E2E_TEST_CONFIG.API_BASE}/files/type/images`);
      const isSuccess = response.ok;
      await this.assert(isSuccess, 'Files list retrieved successfully');
      
      if (isSuccess) {
        const data = await response.json();
        const files = data.files || data || [];
        await this.log(`Found ${files.length} files`, 'info');
        
        // Find our test file
        const testFile = files.find(f => 
          f.name === E2E_TEST_CONFIG.TEST_FILE_NAME || 
          f.fileName === E2E_TEST_CONFIG.TEST_FILE_NAME
        );
        
        if (testFile) {
          await this.log(`Test file found: ${testFile.id || testFile.fileId}`, 'info');
          return testFile.id || testFile.fileId || testFile.hash;
        }
      }
      
      return null;
    } catch (error) {
      this.results.metrics.apiErrors++;
      await this.assert(false, `Get files failed: ${error.message}`);
      return null;
    }
  }

  async testGetNotes(fileId) {
    await this.log('\n=== E2E Test 4: Get Notes for File ===');
    
    if (!fileId) {
      await this.assert(false, 'No file ID available for notes test');
      return [];
    }
    
    try {
      this.results.metrics.apiCalls++;
      const response = await fetch(`${E2E_TEST_CONFIG.API_BASE}/files/${fileId}/notes`);
      const isSuccess = response.ok;
      await this.assert(isSuccess, 'Notes retrieved successfully');
      
      if (isSuccess) {
        const data = await response.json();
        const notes = data.notes || data || [];
        await this.log(`Found ${notes.length} existing notes`, 'info');
        return notes;
      }
      
      return [];
    } catch (error) {
      this.results.metrics.apiErrors++;
      await this.assert(false, `Get notes failed: ${error.message}`);
      return [];
    }
  }

  async testAddNote(fileId) {
    await this.log('\n=== E2E Test 5: Add Note ===');
    
    if (!fileId) {
      await this.assert(false, 'No file ID available for add note test');
      return null;
    }
    
    try {
      this.results.metrics.apiCalls++;
      const response = await fetch(`${E2E_TEST_CONFIG.API_BASE}/files/${fileId}/notes`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          text: E2E_TEST_CONFIG.TEST_NOTE_TEXT
        })
      });
      
      const isSuccess = response.ok;
      await this.assert(isSuccess, 'Note added successfully');
      
      if (isSuccess) {
        const data = await response.json();
        const noteId = data.id || data.noteId;
        await this.log(`Note added with ID: ${noteId}`, 'info');
        return noteId;
      }
      
      return null;
    } catch (error) {
      this.results.metrics.apiErrors++;
      await this.assert(false, `Add note failed: ${error.message}`);
      return null;
    }
  }

  async testDeleteNote(fileId, noteId) {
    await this.log('\n=== E2E Test 6: Delete Note ===');
    
    if (!fileId || !noteId) {
      await this.assert(false, 'No file ID or note ID available for delete test');
      return false;
    }
    
    try {
      this.results.metrics.apiCalls++;
      const response = await fetch(`${E2E_TEST_CONFIG.API_BASE}/files/${fileId}/notes/${noteId}`, {
        method: 'DELETE'
      });
      
      const isSuccess = response.ok;
      await this.assert(isSuccess, 'Note deleted successfully');
      return isSuccess;
    } catch (error) {
      this.results.metrics.apiErrors++;
      await this.assert(false, `Delete note failed: ${error.message}`);
      return false;
    }
  }

  async testMenuActionIntegration() {
    await this.log('\n=== E2E Test 7: Menu Action Integration ===');
    
    // This test would require a browser environment with the actual DOM
    // In a real E2E test, we'd use tools like Playwright or Cypress
    await this.log('Note: Full DOM integration test requires browser environment', 'info');
    await this.log('Use Playwright or Cypress for full E2E testing', 'info');
    
    // Check if the script has the comments case
    try {
      const response = await fetch(`${E2E_TEST_CONFIG.FRONTEND_URL}/src/script.js`);
      if (response.ok) {
        const scriptContent = await response.text();
        const hasCommentsCase = scriptContent.includes("action === 'comments'");
        const hasOpenCommentsModal = scriptContent.includes('openCommentsModal(galleryItem)');
        
        await this.assert(hasCommentsCase, 'Menu action handler includes comments case');
        await this.assert(hasOpenCommentsModal, 'openCommentsModal function is called');
      } else {
        await this.assert(false, 'Could not fetch script.js for verification');
      }
    } catch (error) {
      await this.log(`Could not verify script.js: ${error.message}`, 'warning');
    }
  }

  async calculateMetrics() {
    this.results.metrics.endTime = Date.now();
    this.results.metrics.duration = this.results.metrics.endTime - this.results.metrics.startTime;
    
    const metrics = {
      totalTests: this.results.total,
      passedTests: this.results.passed,
      failedTests: this.results.failed,
      successRate: this.results.total > 0 
        ? Math.round((this.results.passed / this.results.total) * 100) 
        : 0,
      duration: `${(this.results.metrics.duration / 1000).toFixed(2)}s`,
      apiCalls: this.results.metrics.apiCalls,
      apiErrors: this.results.metrics.apiErrors,
      apiSuccessRate: this.results.metrics.apiCalls > 0
        ? Math.round(((this.results.metrics.apiCalls - this.results.metrics.apiErrors) / this.results.metrics.apiCalls) * 100)
        : 0
    };
    
    return metrics;
  }

  async printSummary() {
    const metrics = await this.calculateMetrics();
    
    console.log('\n========================================');
    console.log('E2E Test Summary - Notes Menu Action');
    console.log('========================================');
    console.log(`Total Tests: ${metrics.totalTests}`);
    console.log(`Passed: ${metrics.passedTests}`);
    console.log(`Failed: ${metrics.failedTests}`);
    console.log(`Success Rate: ${metrics.successRate}%`);
    console.log(`Duration: ${metrics.duration}`);
    console.log(`API Calls: ${metrics.apiCalls}`);
    console.log(`API Errors: ${metrics.apiErrors}`);
    console.log(`API Success Rate: ${metrics.apiSuccessRate}%`);
    console.log('========================================\n');
    
    return metrics;
  }

  async runAllTests() {
    console.log('========================================');
    console.log('Notes Menu Action - E2E Tests');
    console.log('========================================\n');
    
    // Test 1: Backend Health
    const isHealthy = await this.testBackendHealth();
    if (!isHealthy) {
      await this.log('Backend is not available. Some tests will be skipped.', 'warning');
    }
    
    // Test 2: Upload Test File
    const fileHash = await this.testUploadTestFile();
    
    // Test 3: Get Files
    const fileId = await this.testGetFiles() || fileHash;
    
    // Test 4: Get Notes
    const existingNotes = await this.testGetNotes(fileId);
    
    // Test 5: Add Note
    const noteId = await this.testAddNote(fileId);
    
    // Test 6: Delete Note (cleanup)
    if (noteId) {
      await this.testDeleteNote(fileId, noteId);
    }
    
    // Test 7: Menu Action Integration
    await this.testMenuActionIntegration();
    
    // Print summary
    const metrics = await this.printSummary();
    
    return {
      results: this.results,
      metrics
    };
  }
}

// Export for use in test runners
if (typeof module !== 'undefined' && module.exports) {
  module.exports = NotesMenuActionE2ETest;
}

// Auto-run in browser environment
if (typeof window !== 'undefined') {
  window.NotesMenuActionE2ETest = NotesMenuActionE2ETest;
  window.runNotesMenuActionE2ETests = async function() {
    const test = new NotesMenuActionE2ETest();
    return await test.runAllTests();
  };
}

// Auto-run in Node.js environment
if (typeof require !== 'undefined' && require.main === module) {
  const test = new NotesMenuActionE2ETest();
  test.runAllTests().then(results => {
    process.exit(results.results.failed > 0 ? 1 : 0);
  }).catch(error => {
    console.error('E2E test error:', error);
    process.exit(1);
  });
}

