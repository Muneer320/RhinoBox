/**
 * Browser Console Connection Test Script
 *
 * Copy and paste this entire script into your browser's developer console
 * (F12 -> Console tab) while on the frontend page (http://localhost:5173)
 *
 * This will test the frontend-backend connection and report any issues.
 */

(async function testConnection() {
  console.log('🔍 Testing Frontend-Backend Connection...\n');

  const API_BASE = new URL("/api", location.origin).href;
  const { API_CONFIG } = await import("/src/api.js");
  const results = {
    passed: [],
    failed: [],
    warnings: []
  };

  // Test 1: Backend Health Check
  console.log('1️⃣ Testing backend health endpoint...');
  try {
    const healthResponse = await fetch(`${API_BASE}/healthz`);
    if (healthResponse.ok) {
      const healthData = await healthResponse.json();
      results.passed.push('✅ Backend health check: Server is running');
      console.log('   Status:', healthResponse.status);
      console.log('   Response:', healthData);

      // Check CORS headers
      const corsHeader = healthResponse.headers.get('Access-Control-Allow-Origin');
      if (corsHeader) {
        results.passed.push(`✅ CORS configured: ${corsHeader}`);
        console.log('   CORS Header:', corsHeader);
      } else {
        results.warnings.push('⚠️ CORS header not found in health check response');
      }
    } else {
      results.failed.push(`❌ Backend health check failed: ${healthResponse.status}`);
    }
  } catch (error) {
    results.failed.push(`❌ Backend health check error: ${error.message}`);
    console.error('   Error:', error);
  }

  console.log('');

  // Test 2: Config Endpoint
  console.log('2️⃣ Testing /api/config endpoint...');
  try {
    const configResponse = await fetch(`${API_BASE}/api/config`, {
      method: 'GET',
      headers: { 'Accept': 'application/json' }
    });

    if (configResponse.ok) {
      const configData = await configResponse.json();
      results.passed.push('✅ Config endpoint: Working correctly');
      console.log('   Status:', configResponse.status);
      console.log('   Config:', configData);

      // Validate config structure
      if (configData.auth_enabled !== undefined) {
        results.passed.push('✅ Config structure: Valid');
      } else {
        results.warnings.push('⚠️ Config structure may be incomplete');
      }
    } else {
      results.failed.push(`❌ Config endpoint failed: ${configResponse.status} ${configResponse.statusText}`);
      console.error('   Response:', await configResponse.text());
    }
  } catch (error) {
    results.failed.push(`❌ Config endpoint error: ${error.message}`);
    console.error('   Error:', error);
  }

  console.log('');

  // Test 3: Frontend API Configuration
  console.log('3️⃣ Testing frontend API configuration...');
  try {
    // Check if API_CONFIG is available (if script.js is loaded)
    if (typeof API_CONFIG !== 'undefined') {
      results.passed.push(`✅ Frontend API_CONFIG found: ${API_CONFIG.baseURL}`);
      console.log('   API Base URL:', API_CONFIG.baseURL);

      if (API_CONFIG.baseURL === API_BASE) {
        results.passed.push('✅ API base URL matches backend');
      } else {
        results.warnings.push(`⚠️ API base URL mismatch: Frontend uses ${API_CONFIG.baseURL}, expected ${API_BASE}`);
      }
    } else {
      results.warnings.push('⚠️ API_CONFIG not found in global scope (may need to import from api.js)');
    }
  } catch (error) {
    results.warnings.push(`⚠️ Could not check API_CONFIG: ${error.message}`);
  }

  console.log('');

  // Test 4: CORS Preflight (OPTIONS request)
  console.log('4️⃣ Testing CORS preflight...');
  try {
    const optionsResponse = await fetch(`${API_BASE}/api/config`, {
      method: 'OPTIONS',
      headers: {
        'Origin': window.location.origin,
        'Access-Control-Request-Method': 'GET',
        'Access-Control-Request-Headers': 'Content-Type'
      }
    });

    if (optionsResponse.status === 204 || optionsResponse.status === 200) {
      results.passed.push('✅ CORS preflight: Working');
      console.log('   Status:', optionsResponse.status);

      const allowOrigin = optionsResponse.headers.get('Access-Control-Allow-Origin');
      const allowMethods = optionsResponse.headers.get('Access-Control-Allow-Methods');

      if (allowOrigin) console.log('   Allow-Origin:', allowOrigin);
      if (allowMethods) console.log('   Allow-Methods:', allowMethods);
    } else {
      results.warnings.push(`⚠️ CORS preflight returned: ${optionsResponse.status}`);
    }
  } catch (error) {
    results.warnings.push(`⚠️ CORS preflight error: ${error.message}`);
  }

  console.log('');

  // Test 5: Network connectivity
  console.log('5️⃣ Testing network connectivity...');
  try {
    const startTime = performance.now();
    const testResponse = await fetch(`${API_BASE}/healthz`);
    const endTime = performance.now();
    const latency = Math.round(endTime - startTime);

    if (testResponse.ok) {
      results.passed.push(`✅ Network latency: ${latency}ms`);
      console.log(`   Response time: ${latency}ms`);

      if (latency > 1000) {
        results.warnings.push('⚠️ High latency detected (>1000ms)');
      }
    }
  } catch (error) {
    results.failed.push(`❌ Network connectivity test failed: ${error.message}`);
  }

  console.log('\n' + '='.repeat(60));
  console.log('📊 TEST RESULTS SUMMARY');
  console.log('='.repeat(60));

  if (results.passed.length > 0) {
    console.log('\n✅ PASSED TESTS:');
    results.passed.forEach(test => console.log('   ' + test));
  }

  if (results.warnings.length > 0) {
    console.log('\n⚠️ WARNINGS:');
    results.warnings.forEach(warning => console.log('   ' + warning));
  }

  if (results.failed.length > 0) {
    console.log('\n❌ FAILED TESTS:');
    results.failed.forEach(failure => console.log('   ' + failure));
  }

  console.log('\n' + '='.repeat(60));

  const totalTests = results.passed.length + results.failed.length;
  const passRate = totalTests > 0 ? (results.passed.length / totalTests * 100).toFixed(1) : 0;
  console.log(`\n📈 Overall: ${results.passed.length}/${totalTests} tests passed (${passRate}%)`);

  if (results.failed.length === 0 && results.warnings.length === 0) {
    console.log('🎉 All tests passed! Frontend and backend are properly connected.');
  } else if (results.failed.length === 0) {
    console.log('✅ All critical tests passed! Some warnings to review.');
  } else {
    console.log('⚠️ Some tests failed. Please review the errors above.');
  }

  return results;
})();

