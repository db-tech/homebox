#!/usr/bin/env sh
# Starts a freshly built Homebox image against an empty volume and checks that
# the schema migrations apply from scratch and the API answers, before the
# image is allowed anywhere near the live database.
#
# Expects: IMAGE_REPO, TEST_TAG, BUILD_NUMBER, BUILDER_IMAGE
set -e

NAME="homebox-verify-${BUILD_NUMBER}"
VOL="homebox-verify-${BUILD_NUMBER}"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker volume rm "$VOL" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker volume create "$VOL" >/dev/null
docker run -d --name "$NAME" -v "$VOL":/data \
  -e HBOX_OPTIONS_ALLOW_REGISTRATION=true \
  "${IMAGE_REPO}:${TEST_TAG}" >/dev/null

# Give the container a moment, then make sure it is actually running. Joining
# the network namespace of a container that already died fails with a docker
# error that says nothing about the real problem.
sleep 3
if [ "$(docker inspect -f '{{.State.Running}}' "$NAME" 2>/dev/null)" != "true" ]; then
  echo "ERROR: the container exited immediately after start"
  echo "--- container logs ---"
  docker logs "$NAME" --tail 200 2>&1 || true
  exit 1
fi

# Sharing the app container's network namespace: a published port would be on
# the host, which the Jenkins container cannot reach on 127.0.0.1.
if ! docker run -i --rm --network "container:${NAME}" "${BUILDER_IMAGE}" \
       bash -s < ci/verify-endpoints.sh; then
  echo "--- container logs ---"
  docker logs "$NAME" --tail 200 2>&1 || true
  exit 1
fi
