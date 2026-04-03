#!/bin/bash
# Deployment verification script for proxy-center

set -e

echo "✓ proxy-center Deployment Verification"
echo "========================================"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

check_status() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $1"
    else
        echo -e "${RED}✗${NC} $1"
        ((ERRORS++))
    fi
}

check_warning() {
    if [ $? -ne 0 ]; then
        echo -e "${YELLOW}⚠${NC} $1"
        ((WARNINGS++))
    fi
}

# 1. Container status
echo "1. Container Status"
echo "-------------------"
docker ps | grep proxy-center > /dev/null
check_status "Container is running"

CONTAINER_STATUS=$(docker inspect proxy-center --format='{{.State.Status}}' 2>/dev/null)
if [ "$CONTAINER_STATUS" = "running" ]; then
    UPTIME=$(docker inspect proxy-center --format='{{.State.StartedAt}}')
    echo "  Started at: $UPTIME"
fi
echo ""

# 2. Port availability
echo "2. Port Availability"
echo "--------------------"
netstat -tuln 2>/dev/null | grep -E "8080|1080|8090" > /dev/null || check_warning "Ports not visible via netstat (check with ss or lsof)"
echo "  Checking Docker port mappings..."
docker port proxy-center 2>/dev/null | grep -E "8080|1080|8090"
echo ""

# 3. Service endpoints
echo "3. Service Endpoints"
echo "--------------------"
curl -s http://localhost:8090/healthz > /dev/null
check_status "Web UI health check"

curl -s -u admin:change-me-now http://localhost:8090/api/users > /dev/null
check_status "API authentication"

echo ""

# 4. Database connectivity
echo "4. Database Connectivity"
echo "------------------------"
docker exec proxy-center test -f /data/proxy-center.db
check_status "Database file exists"

USER_COUNT=$(docker exec proxy-center sqlite3 /data/proxy-center.db "SELECT COUNT(*) FROM users;" 2>/dev/null)
if [ -n "$USER_COUNT" ]; then
    echo "  Users in database: $USER_COUNT"
fi

TABLE_COUNT=$(docker exec proxy-center sqlite3 /data/proxy-center.db ".tables" 2>/dev/null | wc -w)
echo "  Tables: $TABLE_COUNT"
echo ""

# 5. Resource usage
echo "5. Resource Usage"
echo "-----------------"
STATS=$(docker stats --no-stream proxy-center 2>/dev/null)
echo "$STATS" | tail -1
echo ""

# 6. Storage check
echo "6. Storage Check"
echo "----------------"
docker exec proxy-center df -h /data | tail -1
echo ""

# 7. Log analysis
echo "7. Recent Logs"
echo "--------------"
RECENT_LOGS=$(docker logs --tail 10 proxy-center 2>&1 | tail -3)
echo "$RECENT_LOGS"
echo ""

# 8. Network connectivity
echo "8. Network Connectivity"
echo "----------------------"
docker exec proxy-center ping -c 1 8.8.8.8 > /dev/null 2>&1
check_warning "Internet connectivity from container"

echo ""

# Summary
echo "========================================"
echo "Verification Summary"
echo "---"
echo "Errors: $ERRORS"
echo "Warnings: $WARNINGS"

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ Deployment verification passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Deployment has issues!${NC}"
    exit 1
fi
