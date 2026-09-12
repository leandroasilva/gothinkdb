#!/bin/bash
# GoThinkDB DockerHub Build & Push Script
#
# Usage:
#   ./scripts/dockerhub-push.sh [tag]
#
# Examples:
#   ./scripts/dockerhub-push.sh          # Push as :latest
#   ./scripts/dockerhub-push.sh v1.0.0   # Push as :v1.0.0 and :latest

set -e

DOCKERHUB_USER="${DOCKERHUB_USER:-halklenson}"
IMAGE_NAME="${DOCKERHUB_USER}/gothinkdb"
TAG="${1:-latest}"

echo "=========================================="
echo "GoThinkDB DockerHub Build & Push"
echo "=========================================="
echo ""
echo "Image: ${IMAGE_NAME}"
echo "Tag: ${TAG}"
echo ""

# Check if logged in
if ! docker info 2>/dev/null | grep -q "Username"; then
    echo "ERROR: Not logged in to DockerHub"
    echo "Please run: docker login"
    exit 1
fi

# Build the image
echo "Building image..."
docker build -t "${IMAGE_NAME}:${TAG}" .

if [ "${TAG}" != "latest" ]; then
    echo "Also tagging as :latest..."
    docker tag "${IMAGE_NAME}:${TAG}" "${IMAGE_NAME}:latest"
fi

echo ""
echo "Pushing to DockerHub..."
docker push "${IMAGE_NAME}:${TAG}"

if [ "${TAG}" != "latest" ]; then
    docker push "${IMAGE_NAME}:latest"
fi

echo ""
echo "=========================================="
echo "Done!"
echo "=========================================="
echo ""
echo "Pull with: docker pull ${IMAGE_NAME}:${TAG}"
echo ""
echo "Run standalone:"
echo "  docker run -d --name gothinkdb -p 28015:28015 -p 8080:8080 -p 29015:29015 ${IMAGE_NAME}:${TAG}"
echo ""
echo "Run cluster:"
echo "  DOCKERHUB_USER=${DOCKERHUB_USER} DOCKERHUB_TAG=${TAG} docker compose -f docker-compose.cluster.yml up -d"
