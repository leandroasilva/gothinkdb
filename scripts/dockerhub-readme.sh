#!/bin/bash
# GoThinkDB DockerHub README Publisher
# Publishes the DOCKERHUB.md as the repository description on DockerHub
#
# Usage:
#   ./scripts/dockerhub-readme.sh [username] [password]
#
# Or interactively:
#   ./scripts/dockerhub-readme.sh

set -e

DOCKERHUB_USER="${1:-${DOCKERHUB_USER:-halklenson}}"
DOCKERHUB_PASS="${2:-}"
REPO_NAME="gothinkdb"
README_FILE="DOCKERHUB.md"
FULL_DESCRIPTION="GoThinkDB - RethinkDB compatible database in Go"

echo "=========================================="
echo "GoThinkDB DockerHub README Publisher"
echo "=========================================="
echo ""

if [ ! -f "$README_FILE" ]; then
    echo "ERROR: $README_FILE not found"
    exit 1
fi

# If password not provided, ask for it
if [ -z "$DOCKERHUB_PASS" ]; then
    echo "Enter your DockerHub credentials:"
    echo "  Username: $DOCKERHUB_USER"
    read -sp "  Password: " DOCKERHUB_PASS
    echo ""
    echo ""
fi

if [ -z "$DOCKERHUB_PASS" ]; then
    echo "ERROR: Password is required"
    exit 1
fi

# Get DockerHub JWT token
echo "Getting DockerHub authentication token..."
TOKEN_RESPONSE=$(curl -s -X POST https://hub.docker.com/v2/users/login/ \
    -H "Content-Type: application/json" \
    -d "{\"username\": \"${DOCKERHUB_USER}\", \"password\": \"${DOCKERHUB_PASS}\"}")

JWT=$(echo "$TOKEN_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)

if [ -z "$JWT" ]; then
    echo "ERROR: Failed to authenticate"
    echo "Response: $TOKEN_RESPONSE"
    exit 1
fi

echo "Authenticated successfully"
echo ""

# Update repository description and full_description
echo "Updating repository README..."
RESPONSE=$(curl -s -X PATCH "https://hub.docker.com/v2/repositories/${DOCKERHUB_USER}/${REPO_NAME}/" \
    -H "Authorization: JWT ${JWT}" \
    -H "Content-Type: application/json" \
    -d "{
        \"description\": \"${FULL_DESCRIPTION}\",
        \"full_description\": $(python3 -c "import json; print(json.dumps(open('${README_FILE}').read()))")
    }")

echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"

echo ""
echo "=========================================="
echo "Done!"
echo "=========================================="
echo ""
echo "Check your repository at: https://hub.docker.com/r/${DOCKERHUB_USER}/${REPO_NAME}"
