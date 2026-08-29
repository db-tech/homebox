// Build/deploy pipeline for the Homebox fork (homebox.dbiber.de).
//
//   1. Prepare Builder Image  (Go + Node toolchain for the test stages)
//   2. Test                   (backend + frontend, in parallel)
//   3. Build Server Image     (:sha + :latest on main, :previous kept back)
//   4. Verify Image           (fresh volume: migrations apply from scratch)
//   5. Deploy                 (main only, compose up from /tmp/homebox-deploy)
//   6. Smoke                  (curl /api/v1/status, auto-rollback on failure)
//
// Why a builder image, when the production Dockerfile is self-sufficient:
// Jenkins itself runs in a container and talks to the host Docker daemon. A
// bind mount of $WORKSPACE would reference a path that exists inside the
// Jenkins container but not on the host, so the mount would come up empty.
// Letting the docker-workflow plugin run the steps handles that correctly,
// and the plugin starts the container as uid 1000 with no writable $HOME -
// which is what ci/Dockerfile.builder prepares for.
//
// For the same reason the Verify stage cannot curl a published port on
// 127.0.0.1: it shares the test container's network namespace instead.
//
// docker-compose-remote.yml lives in this repo, matching the stoa convention
// that own apps deploy self-contained from /tmp. The older
// /home/wyf/dockerize/homebox/ copy builds from a server-local clone and is
// not used by this pipeline - the Jenkins container cannot even see it.
//
// The homebox_data volume is external and holds the live SQLite database. It
// is never touched by a redeploy or a rollback.
//
// Requirements on the Jenkins host:
//   - Docker socket mounted (already the case)
//   - Network "service-proxy" and volume "homebox_data" exist (both do)
//   - Jenkins file credential 'homebox_env' holding the runtime env
//     (HBOX_ADMIN_*, HBOX_OPTIONS_*, ...). Same shape as stoa_env.
//
// Trigger: Multibranch job config "Periodically if not otherwise run".

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
    BUILDER_IMAGE   = 'homebox-builder:latest'
    COMPOSE_PROJECT = 'homebox'
    COMPOSE_FILE    = 'docker-compose-remote.yml'
    DEPLOY_DIR      = '/tmp/homebox-deploy'
    CONTAINER_NAME  = 'homebox'
    SMOKE_URL       = 'https://homebox.dbiber.de/api/v1/status'

    ENV_CREDENTIAL_ID = 'homebox_env'
  }

  stages {

    stage('Prepare Builder Image') {
      agent any
      steps {
        sh 'docker build -t $BUILDER_IMAGE -f ci/Dockerfile.builder ci/'
      }
    }

    stage('Test') {
      parallel {
        stage('Backend') {
          agent {
            docker {
              image "${BUILDER_IMAGE}"
              reuseNode true
            }
          }
          steps {
            dir('backend') {
              sh '''
                export HOME=/tmp
                set -e
                test -z "$(gofmt -l ./app ./internal ./pkgs)" || {
                  echo "gofmt found unformatted files:"
                  gofmt -l ./app ./internal ./pkgs
                  exit 1
                }
                go vet ./...
                go test ./... -count=1
              '''
            }
          }
        }

        stage('Frontend') {
          agent {
            docker {
              image "${BUILDER_IMAGE}"
              reuseNode true
            }
          }
          steps {
            dir('frontend') {
              sh '''
                export HOME=/tmp
                set -e
                # A workspace reused across builds can carry node_modules built
                # by a different image; start clean.
                rm -rf node_modules
                pnpm install --frozen-lockfile --shamefully-hoist
                pnpm run typecheck
                pnpm run lint
              '''
            }
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

    // Runs the freshly built image against an EMPTY volume, proving the schema
    // migrations apply from scratch and the endpoints answer, before anything
    // goes near the live database. The logic lives in ci/verify-image.sh so it
    // can be run and tested outside Jenkins.
    stage('Verify Image') {
      agent any
      steps {
        script {
          env.TEST_TAG = (env.BRANCH_NAME == 'main') ? env.GIT_SHA : "${env.BRANCH_TAG}-${env.GIT_SHA}"
        }
        sh 'sh ci/verify-image.sh'
      }
    }

    stage('Deploy') {
      when { branch 'main' }
      agent any
      steps {
        // The compose file comes from this repo; the runtime env (with the
        // admin credentials) comes from a Jenkins file credential, because the
        // Jenkins container cannot see /home/wyf on the host.
        //
        // install -m instead of cp: a previous build leaves .env as 0600, and a
        // plain cp then fails on overwrite.
        withCredentials([file(credentialsId: env.ENV_CREDENTIAL_ID, variable: 'ENV_FILE')]) {
          sh '''
            set -e
            mkdir -p "$DEPLOY_DIR"
            install -m 600 "$ENV_FILE" "$DEPLOY_DIR/.env"
            install -m 644 "$COMPOSE_FILE" "$DEPLOY_DIR/"
            cd "$DEPLOY_DIR"
            docker compose -p $COMPOSE_PROJECT -f $COMPOSE_FILE up -d
            echo "Deployed $IMAGE_REPO:latest"
          '''
        }
      }
    }

    stage('Smoke') {
      when { branch 'main' }
      agent any
      steps {
        withCredentials([file(credentialsId: env.ENV_CREDENTIAL_ID, variable: 'ENV_FILE')]) {
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
                mkdir -p "$DEPLOY_DIR"
                install -m 600 "$ENV_FILE" "$DEPLOY_DIR/.env"
                install -m 644 "$COMPOSE_FILE" "$DEPLOY_DIR/"
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
