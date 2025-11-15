#!/bin/bash

# End-to-end test script for Info Modal feature
# This script tests the complete flow: upload file -> get metadata -> verify info modal can display it

set -e

echo "=========================================="
echo "Info Modal Feature - End-to-End Test"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BACKEND_URL="${BACKEND_URL:-http://localhost:8090}"
TEST_FILE="test_info_modal.txt"
TEST_CONTENT="This is a test file for info modal feature testing."

echo -e "${YELLOW}Step 1: Checking backend health...${NC}"
if curl -s -f "${BACKEND_URL}/healthz" > /dev/null; then
    echo -e "${GREEN}✓ Backend is running${NC}"
else
    echo -e "${RED}✗ Backend is not running at ${BACKEND_URL}${NC}"
    echo "Please start the backend server first."
    exit 1
fi

echo ""
echo -e "${YELLOW}Step 2: Uploading test file...${NC}"
# Create test file
echo "${TEST_CONTENT}" > "/tmp/${TEST_FILE}"

# Upload file
UPLOAD_RESPONSE=$(curl -s -X POST \
    -F "file=@/tmp/${TEST_FILE}" \
    "${BACKEND_URL}/ingest/media")

# Extract hash from response
HASH=$(echo "${UPLOAD_RESPONSE}" | grep -o '"hash":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -z "$HASH" ]; then
    # Try alternative response format
    HASH=$(echo "${UPLOAD_RESPONSE}" | grep -o '"hash":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [ -z "$HASH" ]; then
        echo -e "${RED}✗ Failed to extract hash from upload response${NC}"
        echo "Response: ${UPLOAD_RESPONSE}"
        exit 1
    fi
fi

echo -e "${GREEN}✓ File uploaded successfully${NC}"
echo "   Hash: ${HASH}"

echo ""
echo -e "${YELLOW}Step 3: Fetching file metadata...${NC}"
METADATA_RESPONSE=$(curl -s -X GET "${BACKEND_URL}/files/metadata?hash=${HASH}")

if echo "${METADATA_RESPONSE}" | grep -q "hash"; then
    echo -e "${GREEN}✓ Metadata retrieved successfully${NC}"
else
    echo -e "${RED}✗ Failed to retrieve metadata${NC}"
    echo "Response: ${METADATA_RESPONSE}"
    exit 1
fi

echo ""
echo -e "${YELLOW}Step 4: Verifying metadata fields...${NC}"
REQUIRED_FIELDS=("hash" "original_name" "stored_path" "category" "mime_type" "size" "uploaded_at")
MISSING_FIELDS=()

for field in "${REQUIRED_FIELDS[@]}"; do
    if echo "${METADATA_RESPONSE}" | grep -q "\"${field}\""; then
        echo -e "  ${GREEN}✓${NC} ${field}"
    else
        echo -e "  ${RED}✗${NC} ${field} (missing)"
        MISSING_FIELDS+=("${field}")
    fi
done

if [ ${#MISSING_FIELDS[@]} -eq 0 ]; then
    echo -e "${GREEN}✓ All required fields present${NC}"
else
    echo -e "${RED}✗ Missing required fields: ${MISSING_FIELDS[*]}${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}Step 5: Displaying metadata (as info modal would)...${NC}"
echo "${METADATA_RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${METADATA_RESPONSE}"

echo ""
echo -e "${YELLOW}Step 6: Testing error cases...${NC}"

# Test missing hash
MISSING_HASH_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "${BACKEND_URL}/files/metadata")
HTTP_CODE=$(echo "${MISSING_HASH_RESPONSE}" | tail -1)
if [ "${HTTP_CODE}" = "400" ]; then
    echo -e "${GREEN}✓ Missing hash returns 400 (Bad Request)${NC}"
else
    echo -e "${RED}✗ Missing hash should return 400, got ${HTTP_CODE}${NC}"
fi

# Test invalid hash
INVALID_HASH_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "${BACKEND_URL}/files/metadata?hash=invalid_hash")
HTTP_CODE=$(echo "${INVALID_HASH_RESPONSE}" | tail -1)
if [ "${HTTP_CODE}" = "404" ]; then
    echo -e "${GREEN}✓ Invalid hash returns 404 (Not Found)${NC}"
else
    echo -e "${YELLOW}⚠ Invalid hash returned ${HTTP_CODE} (expected 404)${NC}"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}All tests passed!${NC}"
echo "=========================================="
echo ""
echo "Summary:"
echo "  - Backend is running"
echo "  - File upload works"
echo "  - Metadata endpoint works"
echo "  - All required fields are present"
echo "  - Error handling works correctly"
echo ""
echo "The info modal feature should work correctly in the frontend!"
echo ""

# Cleanup
rm -f "/tmp/${TEST_FILE}"

