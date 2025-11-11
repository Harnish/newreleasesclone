# GitLab CI/CD Setup Guide

## Overview

Complete GitLab CI/CD pipeline for building, testing, pushing, and deploying the Release Tracker application.

## Pipeline Stages

### 1. **Build Stage**
Builds OCI images using Buildah and Podman in parallel.

**Jobs**:
- `build_with_buildah`: Uses Buildah for efficient building
- `build_with_podman`: Uses Podman for Docker-compatible builds

**Triggers**: On all branches and tags

**Output**: Docker images tagged with commit SHA and `latest`

### 2. **Test Stage**
Comprehensive testing including unit tests, linting, and security scanning.

**Jobs**:
- `test_unit`: Go unit tests with race detection and coverage reporting
- `test_container`: Tests the built container
- `lint`: Code quality checks with golangci-lint
- `security_scan`: Vulnerability scanning with Trivy

**Triggers**: On all branches and tags

**Coverage**: Generates coverage reports visible in GitLab

### 3. **Push Stage**
Pushes images to registries.

**Jobs**:
- `push_to_registry`: Automatic push to GitLab Registry on main/master
- `push_to_dockerhub`: Manual push to Docker Hub on tags

**Triggers**: 
- GitLab Registry: Automatic on main/master/tags
- Docker Hub: Manual on tags

### 4. **Deploy Stage**
Deploys to staging and production environments.

**Jobs**:
- `deploy_staging`: Manual deployment to staging
- `deploy_production`: Manual deployment to production
- `release`: Creates GitLab release on tags

**Triggers**: Manual via GitLab UI

## Setup Instructions

### 1. Push Project to GitLab

```bash
git remote add gitlab https://gitlab.com/yourusername/newreleases.git
git push gitlab main
```

### 2. Configure CI/CD Variables

**Go to**: GitLab Project → **Settings > CI/CD > Variables**

**Add these variables** (toggle "Protected" and "Masked" for security):

#### Required for Docker Hub Push
```
DOCKERHUB_USER          = your-dockerhub-username
DOCKERHUB_TOKEN         = your-dockerhub-token      [Masked] [Protected]
```

#### Required for Deployment
```
STAGING_HOST            = staging.example.com        [Protected]
STAGING_USER            = deploy                     [Protected]
STAGING_PATH            = /app/newreleases           [Protected]

PRODUCTION_HOST         = prod.example.com           [Protected]
PRODUCTION_USER         = deploy                     [Protected]
PRODUCTION_PATH         = /app/newreleases           [Protected]

SSH_PRIVATE_KEY         = (your-deploy-key)          [Masked] [Protected]
```

### 3. Configure Docker Hub Token

**Get token**:
1. Go to https://hub.docker.com/settings/security
2. Create new access token
3. Copy token

**Add to GitLab**:
```
DOCKERHUB_TOKEN = (paste your token)
```

### 4. Configure SSH Key for Deployment

**Generate SSH key** (if needed):
```bash
ssh-keygen -t ed25519 -f ~/.ssh/deploy_key -N ""
```

**Add to GitLab**:
1. GitLab Project → **Settings > Deploy Keys**
2. Paste public key (`deploy_key.pub`)
3. Check "Grant write permissions"

**Add to CI/CD Variables**:
```
SSH_PRIVATE_KEY = (content of deploy_key)
```

### 5. Enable Container Registry

**Go to**: GitLab Project → **Settings > Integrations > Container Registry**

Ensure it's enabled (usually default). Your images will be available at:
```
registry.gitlab.com/yourusername/project-name:tag
```

## Running the Pipeline

### Automatic Pipeline

Push to trigger pipeline:
```bash
git commit -m "Your changes"
git push gitlab main
```

**Pipeline runs automatically**:
1. Build stage (parallel): Buildah + Podman
2. Test stage (parallel): Tests, lint, security
3. Push stage: GitLab Registry push (if main/master)

### Manual Operations

**Push to Docker Hub**:
1. Create a git tag: `git tag v1.0.0 && git push gitlab v1.0.0`
2. Go to **CI/CD > Pipelines**
3. Find the pipeline for your tag
4. Click `push_to_dockerhub` job
5. Click **Play** button

**Deploy to Staging**:
1. Go to **CI/CD > Pipelines**
2. Find pipeline for your branch
3. Click `deploy_staging` job
4. Click **Play** button
5. Watch logs in real-time

**Deploy to Production**:
1. Go to **CI/CD > Pipelines**
2. Find pipeline for your release tag
3. Click `deploy_production` job
4. Click **Play** button

## Pipeline Configuration

### Environment Variables

```yaml
REGISTRY: registry.gitlab.com
IMAGE_NAME: ${REGISTRY}/${CI_PROJECT_PATH}
```

These are automatically set. The full image name becomes:
```
registry.gitlab.com/yourusername/project-name:tag
```

### Build Tags

Images are automatically tagged with:
```
${IMAGE_NAME}:${CI_COMMIT_SHORT_SHA}  # Short commit hash
${IMAGE_NAME}:latest                   # Latest tag
${IMAGE_NAME}:${CI_COMMIT_TAG}         # Version tag (on tags)
```

## Viewing Pipeline Status

**In GitLab UI**:

1. **Overview Dashboard**
   - Project homepage shows pipeline status
   - Quick view of last pipeline

2. **CI/CD > Pipelines**
   - View all pipelines
   - Click pipeline to see jobs
   - Click job to see logs

3. **CI/CD > Jobs**
   - View all jobs across pipelines
   - See individual job status and logs

4. **Deployments**
   - View deployment history
   - Rollback to previous deployments

## Logs and Debugging

### View Job Logs

1. Go to **CI/CD > Pipelines**
2. Click pipeline
3. Click job name
4. View full logs in real-time

### Common Issues

**Build Fails**:
- Check Dockerfile is in root directory
- Ensure go.mod and main.go exist
- Review build logs for errors

**Tests Fail**:
- Run locally: `go test -v -race ./...`
- Check that all dependencies are available
- Review test logs in pipeline

**Push Fails**:
- Verify DOCKERHUB_TOKEN is set correctly
- Check SSH key has correct permissions
- Ensure repository exists at destination

**Deploy Fails**:
- Verify SSH_PRIVATE_KEY is set correctly
- Check staging/production servers are accessible
- Ensure deploy user has correct permissions
- Review SSH connection in logs

## Advanced Usage

### Skip Pipeline

Add to commit message:
```
git commit -m "Your changes [skip ci]"
```

### Only Specific Branch

Modify `.gitlab-ci.yml`:
```yaml
job_name:
  only:
    - main
```

### Run on Schedule

**Settings > CI/CD > Schedules**

Create scheduled pipelines for periodic builds.

### Auto-Deployment

Change `when: manual` to `when: on_success`:
```yaml
deploy_staging:
  when: on_success  # Automatic after build
```

### Environment Protection

**Settings > Deployments > Environments**

Restrict production deployments to main branch.

## Performance Optimization

### Parallel Builds

Buildah and Podman run in parallel by default:
```yaml
stages:
  - build  # Buildah and Podman run simultaneously
  - test
```

### Cache Dependencies

Add to jobs:
```yaml
cache:
  paths:
    - vendor/
```

### Artifacts Retention

Control how long to keep artifacts:
```yaml
artifacts:
  expire_in: 30 days
```

## Deployment Best Practices

✅ **Use Protected Environments**
- Restrict production to main branch
- Require approvals for production deployments

✅ **Use SSH Keys**
- Don't use passwords
- Use deploy keys with limited permissions

✅ **Monitor Deployments**
- Check logs after deployment
- Set up rollback procedure

✅ **Version Tagging**
- Use semantic versioning: v1.0.0, v1.1.0, etc.
- Tag releases in git

✅ **Test Before Deploy**
- All jobs must pass before deployment
- Run integration tests in pipeline

## Connecting Other Services

### Slack Notifications

**Settings > Integrations > Slack Notifications**

Get notified of pipeline status in Slack.

### Email Notifications

**User Settings > Notifications > Pipeline**

Get email on pipeline failures/success.

### Webhooks

**Settings > Webhooks**

Send pipeline events to external services.

## Troubleshooting

### "YAML Error" in Pipeline

- Check `.gitlab-ci.yml` syntax
- Use GitLab CI/CD Lint: **CI/CD > Pipelines > CI/CD Lint**
- Ensure proper indentation (2 spaces)

### "Registry login failed"

- Verify DOCKERHUB_TOKEN is correct
- Check Docker Hub token has access
- Ensure CI/CD variables are set as Protected

### "SSH connection failed"

- Verify SSH_PRIVATE_KEY is set
- Check public key is in ~/.ssh/authorized_keys on server
- Ensure staging/production servers are accessible

### "Artifact not found"

- Check artifact path in job
- Verify previous job completed successfully
- Check artifact hasn't expired

## Resources

- **GitLab CI/CD Docs**: https://docs.gitlab.com/ee/ci/
- **GitLab CI/CD Variables**: https://docs.gitlab.com/ee/ci/variables/
- **Container Registry**: https://docs.gitlab.com/ee/user/packages/container_registry/
- **Deploy Keys**: https://docs.gitlab.com/ee/user/project/deploy_keys/
- **Protected Variables**: https://docs.gitlab.com/ee/ci/variables/#protected-variables

## Summary

The GitLab CI/CD pipeline provides:
- ✅ Automatic builds on every push
- ✅ Comprehensive testing
- ✅ Multi-registry push support
- ✅ Easy deployment to staging/production
- ✅ Automatic release creation
- ✅ Security scanning
- ✅ Coverage reporting

---

**Created**: November 10, 2025
**Status**: Production Ready
**Compatible with**: GitLab Community Edition and GitLab.com
