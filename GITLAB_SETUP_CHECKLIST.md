# 📋 GitLab CI/CD Setup Checklist

## Pre-Setup Requirements

- [ ] Project ready to push to GitLab
- [ ] Go 1.24.9+ installed (for local testing)
- [ ] Git configured and ready
- [ ] GitLab account created
- [ ] GitLab project created (or ready to create)

---

## Step 1: Push Project to GitLab

```bash
# Add GitLab remote
git remote add gitlab https://gitlab.com/yourusername/newreleases.git

# Push to GitLab
git push gitlab main
```

**Verify**:
- [ ] Project appears in GitLab
- [ ] All files are present
- [ ] `.gitlab-ci.yml` is in root directory

---

## Step 2: Configure CI/CD Variables

**Go to**: GitLab Project → **Settings > CI/CD > Variables**

### Required Variables

#### DOCKERHUB_USER
- **Type**: Variable
- **Value**: Your Docker Hub username
- **Protected**: Optional
- **Masked**: No

#### DOCKERHUB_TOKEN
- **Type**: Variable  
- **Value**: Your Docker Hub token (create at https://hub.docker.com/settings/security)
- **Protected**: Yes ✅
- **Masked**: Yes ✅

#### SSH_PRIVATE_KEY
- **Type**: Variable
- **Value**: Contents of your SSH private key (for deployment)
- **Protected**: Yes ✅
- **Masked**: Yes ✅

```bash
# To get SSH key content:
cat ~/.ssh/deploy_key
# Then paste into GitLab variable
```

### Deployment Variables (Required for deployment)

#### STAGING_HOST
- **Type**: Variable
- **Value**: `staging.example.com`
- **Protected**: Yes ✅

#### STAGING_USER
- **Type**: Variable
- **Value**: SSH user for staging (e.g., `deploy`)
- **Protected**: Yes ✅

#### STAGING_PATH
- **Type**: Variable
- **Value**: `/app/newreleases`
- **Protected**: Yes ✅

### Optional: Production Variables

#### PRODUCTION_HOST
- **Type**: Variable
- **Value**: `prod.example.com`
- **Protected**: Yes ✅

#### PRODUCTION_USER
- **Type**: Variable
- **Value**: SSH user for production (e.g., `deploy`)
- **Protected**: Yes ✅

#### PRODUCTION_PATH
- **Type**: Variable
- **Value**: `/app/newreleases`
- **Protected**: Yes ✅

**Verify**:
- [ ] All required variables are set
- [ ] Sensitive variables are marked as Protected
- [ ] Sensitive variables are marked as Masked
- [ ] Variable values are correct

---

## Step 3: Verify Container Registry

**Go to**: GitLab Project → **Settings > General > Container Registry**

**Verify**:
- [ ] Container Registry is enabled (usually default)
- [ ] If not enabled, click "Enable" button

---

## Step 4: Configure Deploy SSH Key (if deploying)

**Generate SSH key** (if needed):
```bash
ssh-keygen -t ed25519 -f ~/.ssh/deploy_key -C "deploy@$(hostname)" -N ""
```

**On deployment server**:
```bash
# Create deploy user (if needed)
sudo useradd -m deploy

# Add public key to authorized_keys
sudo -u deploy mkdir -p /home/deploy/.ssh
sudo -u deploy chmod 700 /home/deploy/.ssh
cat ~/.ssh/deploy_key.pub | sudo -u deploy tee -a /home/deploy/.ssh/authorized_keys
sudo -u deploy chmod 600 /home/deploy/.ssh/authorized_keys

# Test SSH access
ssh -i ~/.ssh/deploy_key deploy@staging.example.com "whoami"
```

**In GitLab**:
```bash
# Add SSH_PRIVATE_KEY to CI/CD variables
cat ~/.ssh/deploy_key | pbcopy  # macOS
# or
cat ~/.ssh/deploy_key  # Linux - then copy manually
```

**Verify**:
- [ ] SSH key generated
- [ ] Public key added to deployment servers
- [ ] SSH_PRIVATE_KEY set in GitLab variables
- [ ] Can SSH to deployment servers

---

## Step 5: First Pipeline Run

**Trigger**:
```bash
# Make a commit and push
git commit --allow-empty -m "Trigger pipeline"
git push gitlab main
```

**Monitor**:
1. Go to GitLab: **CI/CD > Pipelines**
2. Click on the new pipeline
3. Watch jobs run in real-time
4. Check logs for any errors

**Verify**:
- [ ] Pipeline triggers automatically
- [ ] Build stage completes
- [ ] Test stage completes
- [ ] Push stage completes
- [ ] No errors in logs

---

## Step 6: Deploy (Manual)

### Deploy to Staging

**Via GitLab UI**:
1. Go to **CI/CD > Pipelines**
2. Find pipeline for your branch
3. Click on the pipeline
4. Find `deploy_staging` job
5. Click **Play** button
6. Wait for deployment to complete

**Verify**:
- [ ] Deployment job runs
- [ ] Logs show successful deployment
- [ ] Can access staging at http://staging.example.com:8080

### Deploy to Production

**Via GitLab UI** (same as staging):
1. Go to **CI/CD > Pipelines**
2. Find pipeline for your **tag** (not branch)
3. Click on the pipeline
4. Find `deploy_production` job
5. Click **Play** button
6. Wait for deployment to complete

**Verify**:
- [ ] Deployment job runs
- [ ] Logs show successful deployment
- [ ] Can access production at http://prod.example.com:8080

---

## Step 7: Push to Docker Hub (Optional)

**Requirements**:
- [ ] DOCKERHUB_USER set in variables
- [ ] DOCKERHUB_TOKEN set in variables
- [ ] Release tagged (v1.0.0 format)

**Trigger**:
```bash
# Create a tag and push
git tag v1.0.0
git push gitlab v1.0.0
```

**Via GitLab UI**:
1. Go to **CI/CD > Pipelines**
2. Find pipeline for your **tag**
3. Click on the pipeline
4. Find `push_to_dockerhub` job
5. Click **Play** button
6. Wait for push to complete

**Verify**:
- [ ] Docker Hub push job runs
- [ ] Image appears in Docker Hub
- [ ] Image tags are correct (v1.0.0, latest)

---

## Step 8: Verify Automated Release (on Tags)

**Verify**:
- [ ] Go to **Deployments > Releases**
- [ ] Release is created for your tag
- [ ] Release notes are populated
- [ ] Image URLs are shown

---

## Troubleshooting Checklist

### Pipeline Doesn't Trigger
- [ ] `.gitlab-ci.yml` is in root directory
- [ ] Check pipeline is not disabled (Settings > CI/CD)
- [ ] Check runner is available (CI/CD > Runners)
- [ ] Try: `git push -f` or make a new commit

### Build Fails
- [ ] Dockerfile exists in root
- [ ] go.mod exists in root
- [ ] main.go exists in root
- [ ] Check build logs for specific error
- [ ] Try: `docker build .` locally

### Tests Fail
- [ ] Run tests locally: `go test -v ./...`
- [ ] Check all dependencies available
- [ ] Check for new code errors
- [ ] Review test logs for failures

### Push to Registry Fails
- [ ] DOCKERHUB_TOKEN is correct
- [ ] DOCKERHUB_USER is correct
- [ ] Docker Hub token has access
- [ ] Check push logs for auth errors

### Deployment Fails
- [ ] SSH_PRIVATE_KEY is correct
- [ ] STAGING_HOST is reachable
- [ ] Deploy user has SSH access
- [ ] Check SSH logs: `ssh -v`
- [ ] Check remote deployment logs

### Container Registry Access Fails
- [ ] Container Registry is enabled
- [ ] Check project namespace
- [ ] Try: `podman login registry.gitlab.com`

---

## Quick Command Reference

```bash
# View CI/CD variables
cat /home/jharnish/Work/newreleases/.gitlab-ci.yml

# Trigger pipeline
git commit --allow-empty -m "trigger"
git push gitlab main

# Create release
git tag v1.0.0
git push gitlab v1.0.0

# Test locally
go test -v -race ./...

# Build Docker image
docker build -t newreleases:test .

# Build with Buildah/Podman
./build-podman.sh

# View pipeline logs
# Go to: GitLab > CI/CD > Pipelines > Click pipeline > Click job
```

---

## Documentation Links

- **Setup Guide**: `GITLAB_CI_CD_SUMMARY.md`
- **Complete Guide**: `GITLAB_CI_CD.md`
- **Pipeline Config**: `.gitlab-ci.yml`
- **Docker Guide**: `DOCKER.md`
- **Buildah/Podman**: `BUILDAH_PODMAN.md`
- **Quick Links**: `QUICK_LINKS.md`
- **All Docs**: `PROJECT_INDEX.md`

---

## Success Indicators

✅ All checks complete when:
- [ ] Pipeline triggers automatically on push
- [ ] All 12 jobs run successfully
- [ ] Tests pass (19/19)
- [ ] Image pushed to GitLab Registry
- [ ] Can manually deploy to staging
- [ ] Staging deployment works
- [ ] Can manually deploy to production
- [ ] Production deployment works
- [ ] Can manually push to Docker Hub (on tags)
- [ ] Release is created automatically (on tags)

---

## Next Steps After Setup

1. **Monitor Pipeline**: Watch it run daily
2. **Review Logs**: Keep an eye on job logs
3. **Test Deployments**: Deploy to staging regularly
4. **Tag Releases**: Use semantic versioning (v1.0.0)
5. **Monitor Production**: Watch production after deployments

---

**Created**: November 10, 2025  
**Status**: Ready to Use  
**Estimated Setup Time**: 15-30 minutes

✅ **You're ready to go!**
