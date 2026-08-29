// Build/deploy pipeline for the Homebox fork (homebox.dbiber.de).
//
// Follows the same shape as the stoa/loomaboard pipelines on this Jenkins:
//   1. Test          (Go + frontend, in parallel)
//   2. Build image   (:sha + :latest on main, :previous kept for rollback)
//   3. Verify image  (fresh volume, proves migrations apply from scratch)
//   4. Deploy        (main only, compose up from /tmp/homebox-deploy)
//   5. Smoke         (curl /api/v1/status, auto-rollback on failure)
//
// Homebox-specific notes:
//   - The Dockerfile is multi-stage and self-sufficient, so unlike stoa there
//     is no ci/Dockerfile.builder to prepare. The test stages run in stock
//     golang/node images; the agent only needs Docker.
//   - docker-compose-remote.yml lives in THIS repo, matching the stoa
//     convention ("eigene Apps deployen self-contained aus /tmp/"). The older
//     /home/wyf/dockerize/homebox/ copy builds from a server-local clone and
//     is not used by this pipeline.
//   - The homebox_data volume is external and holds the live SQLite database.
//     It is never touched by a redeploy or a rollback.
//   - Health endpoint is /api/v1/status (not /health) and answers with JSON.
//
// Requirements on the Jenkins host:
//   - Docker socket mounted
//   - Network "service-proxy" exists (nginx-proxy + acme-companion)
//   - Volume "homebox_data" exists
//   - Env file at $DEPLOY_ENV_FILE (see the Deploy stage for the alternative
//     of using a Jenkins file credential instead)
//
// Trigger: Multibranch job config "Periodically if not otherwise run" (1h),
// optionally a webhook to $JENKINS_URL/git/notifyCommit?url=... for lower
// latency.

pipeline {
  agent none

  options {
    timestamps()
    timeout(time: 40, unit: 'MINUTES')
    disableConcurrentBuilds()
    buildDiscarder(logRotator(numToKeepStr: '20', artifactNumToKeepStr: '5'))
  }

  environment {
    DOCKER_BUILDKIT          = '1'
    COMPOSE_DOCKER_CLI_BUILD = '1'

    IMAGE_REPO      = 'biber-tech/homebox'
    COMPOSE_PROJECT = 'homebox'
    COMPOSE_FILE    = 'docker-compose-remote.yml'
    DEPLOY_DIR      = '/tmp/homebox-deploy'
    CONTAINER_NAME  = 'homebox'
    SMOKE_URL       = 'https://homebox.dbiber.de/api/v1/status'

    // Runtime env (contains HBOX_ADMIN_*). Kept out of the repo.
    DEPLOY_ENV_FILE = '/home/wyf/dockerize/homebox/remote.env'

    // Images used for the test stages.
    GO_IMAGE   = 'golang:alpine'
    NODE_IMAGE = 'node:lts-alpine'
  }

  stages {

    stage('Test') {
      parallel {
        stage('Backend') {
          agent any
          steps {
            // The production binary is built CGO-free, but the repo's own tests
            // use mattn/go-sqlite3, which needs a C toolchain.
            sh '''
              set -e
              docker run --rm \
                -v "$WORKSPACE":/src -w /src/backend \
                -e GOFLAGS=-buildvcs=false \
                $GO_IMAGE sh -c '
                  set -e
                  apk add --no-cache build-base git >/dev/null
                  test -z "$(gofmt -l ./app ./internal ./pkgs)" || {
                    echo "gofmt found unformatted files:"
                    gofmt -l ./app ./internal ./pkgs
                    exit 1
                  }
                  go vet ./...
                  go test ./... -count=1
                '
            '''
          }
        }

        stage('Frontend') {
          agent any
          steps {
            sh '''
              set -e
              docker run --rm \
                -e CI=true \
                -v "$WORKSPACE":/src -w /src/frontend \
                $NODE_IMAGE sh -c '
                  set -e
                  # Pinned: the lockfile is v9 and pnpm 10 refuses to run the
                  # build scripts this project needs.
                  npm install -g pnpm@9 >/dev/null
                  pnpm install --frozen-lockfile --shamefully-hoist
                  pnpm run typecheck
                  pnpm run lint
                '
            '''
          }
        }
      }
    }

    stage('Build Server Image') {
      agent any
      steps {
        script {
          env.GIT_SHA    = sh(returnStdout: true, script: 'git rev-parse --short HEAD').trim()
          env.BRANCH_TAG = env.BRANCH_NAME.replaceAll('[^a-zA-Z0-9._-]', '-')
        }
        sh '''
          set -e
          BUILD_ARGS="--build-arg COMMIT=$GIT_SHA --build-arg VERSION=$BRANCH_TAG --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)"

          if [ "$BRANCH_NAME" = "main" ]; then
            # Production build: keep a rollback reserve, then move :latest.
            if docker image inspect $IMAGE_REPO:latest >/dev/null 2>&1; then
              docker tag $IMAGE_REPO:latest $IMAGE_REPO:previous
              echo "Saved current :latest as :previous"
            else
              echo "No previous :latest to save"
            fi
            docker build $BUILD_ARGS -t $IMAGE_REPO:$GIT_SHA -t $IMAGE_REPO:latest .
            echo "Built $IMAGE_REPO:$GIT_SHA + :latest (production)"
          else
            # Branch build: validation only, :latest untouched.
            docker build $BUILD_ARGS -t $IMAGE_REPO:$BRANCH_TAG-$GIT_SHA .
            echo "Built $IMAGE_REPO:$BRANCH_TAG-$GIT_SHA (validation only)"
          fi
        '''
      }
    }

    // Runs the freshly built image against an EMPTY volume. This proves the
    // schema migrations apply from scratch and the endpoints answer, before
    // anything goes near the live database.
    stage('Verify Image') {
      agent any
      steps {
        script {
          env.TEST_TAG = (env.BRANCH_NAME == 'main') ? env.GIT_SHA : "${env.BRANCH_TAG}-${env.GIT_SHA}"
        }
        sh '''
          set -e
          NAME=homebox-verify-$BUILD_NUMBER
          VOL=homebox-verify-$BUILD_NUMBER

          cleanup() {
            docker rm -f "$NAME" >/dev/null 2>&1 || true
            docker volume rm "$VOL" >/dev/null 2>&1 || true
          }
          trap cleanup EXIT

          docker volume create "$VOL" >/dev/null
          docker run -d --name "$NAME" -v "$VOL":/data -P \
            -e HBOX_OPTIONS_ALLOW_REGISTRATION=true \
            $IMAGE_REPO:$TEST_TAG >/dev/null

          PORT=$(docker port "$NAME" 7745/tcp | head -1 | sed 's/.*://')
          BASE="http://127.0.0.1:$PORT/api/v1"

          up=0
          for i in $(seq 1 60); do
            sleep 2
            if curl -sf "$BASE/status" >/dev/null 2>&1; then up=1; break; fi
          done
          if [ "$up" -ne 1 ]; then
            echo "ERROR: container did not become healthy"
            docker logs "$NAME" --tail 200
            exit 1
          fi

          curl -sf -X POST "$BASE/users/register" \
            -H 'Content-Type: application/json' \
            -d '{"name":"ci","email":"ci@example.com","password":"ci-verify-password"}' >/dev/null

          TOKEN=$(curl -sf -X POST "$BASE/users/login" \
            -H 'Content-Type: application/json' \
            -d '{"username":"ci@example.com","password":"ci-verify-password"}' \
            | sed -n 's/.*"token":"\\([^"]*\\)".*/\\1/p')

          if [ -z "$TOKEN" ]; then
            echo "ERROR: could not obtain a token"
            docker logs "$NAME" --tail 200
            exit 1
          fi

          for ep in "pantry/expiring" "pantry/low-stock" "pantry/consumption/statistics"; do
            code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: $TOKEN" "$BASE/$ep")
            echo "  $ep -> $code"
            if [ "$code" != "200" ]; then
              docker logs "$NAME" --tail 200
              exit 1
            fi
          done
          echo "Image verified on a fresh database"
        '''
      }
    }

    stage('Deploy') {
      when { branch 'main' }
      agent any
      steps {
        // The compose file comes from this repo; the env file holds the
        // secrets and stays on the host.
        //
        // To switch to a Jenkins file credential instead (the stoa/loomaboard
        // way), wrap this step in:
        //   withCredentials([file(credentialsId: 'homebox_env', variable: 'ENV_FILE')]) { ... }
        // and use "$ENV_FILE" in place of "$DEPLOY_ENV_FILE".
        //
        // install -m instead of cp: a previous build leaves .env as 0600, and
        // a plain cp then fails on overwrite.
        sh '''
          set -e
          if [ ! -f "$DEPLOY_ENV_FILE" ]; then
            echo "ERROR: env file not found at $DEPLOY_ENV_FILE"
            exit 1
          fi

          mkdir -p "$DEPLOY_DIR"
          install -m 600 "$DEPLOY_ENV_FILE" "$DEPLOY_DIR/.env"
          install -m 644 "$COMPOSE_FILE" "$DEPLOY_DIR/"

          cd "$DEPLOY_DIR"
          docker compose -p $COMPOSE_PROJECT -f $COMPOSE_FILE up -d
          echo "Deployed $IMAGE_REPO:latest"
        '''
      }
    }

    stage('Smoke') {
      when { branch 'main' }
      agent any
      steps {
        script {
          def smokeOk = sh(returnStatus: true, script: '''
            for i in 1 2 3 4 5 6 7 8; do
              sleep 4
              if curl -sf $SMOKE_URL | grep -q '"health":true'; then
                echo "Smoke OK"
                exit 0
              fi
              echo "Smoke attempt $i failed, retrying..."
            done
            echo "ERROR: Smoke failed after 8 attempts"
            docker logs $CONTAINER_NAME --tail 100 || true
            exit 1
          ''') == 0

          if (!smokeOk) {
            echo "==> Smoke failed, attempting auto-rollback"
            sh '''
              set -e
              if ! docker image inspect $IMAGE_REPO:previous >/dev/null 2>&1; then
                echo "No :previous tag yet - cannot roll back automatically"
                exit 1
              fi
              docker tag $IMAGE_REPO:previous $IMAGE_REPO:latest
              cd "$DEPLOY_DIR"
              docker compose -p $COMPOSE_PROJECT -f $COMPOSE_FILE up -d
              sleep 5
              if curl -sf $SMOKE_URL | grep -q '"health":true'; then
                echo "Rollback OK - previous version restored"
              else
                echo "Rollback ran but health still failing - manual intervention required"
              fi
            '''
            error('Build failed: smoke check failed (auto-rollback executed)')
          }
        }
      }
    }

  }

  post {
    success {
      echo "Build #${env.BUILD_NUMBER} on ${env.BRANCH_NAME} succeeded."
    }
    failure {
      echo "Build #${env.BUILD_NUMBER} on ${env.BRANCH_NAME} FAILED."
    }
  }
}
