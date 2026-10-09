# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users
Self-hosters and homelab developers running their own instance (Docker/Kubernetes), sharing it with a few people. Job: know when upstream software they depend on ships a release, so they can decide when to upgrade. Desktop and phone both matter.

## Product Purpose
Release Tracker watches releases across GitHub, GitLab, NPM, PyPI, Docker Hub, and Helm charts in one feed. Success: a user never misses a relevant release and never has to visit each project page.

## Positioning
One self-hosted tracker for many platforms, with the user owning the data. Notifies via browser push, email (immediate or daily digest), and signed webhooks, and can mirror tracked repos to the user's own GitLab with an auto-generated "Awesome" README.

## Operating Context
Multi-user with session auth; email is the user ID and must be verified. Releases are shared between users who track the same repo. Stale projects (>30 min) refresh in the background on page load. SMTP is optional; email features are hidden when unconfigured.

## Capabilities and Constraints
- Go backend, SQLite (`modernc.org/sqlite`), frontend embedded as strings in `ui.go`; no build step or bundler.
- Per-project settings: push, email, webhooks, GitLab sync.
- Releases list: 5 most recent inline, expandable; paginated projects (5/10/20).
- Open decisions: prerelease filtering, sort options, project icons, wide-screen layout (see TODO.md).

## Evidence on Hand
Working app and README.md/TODO.md. No testimonials, customers, or benchmarks exist; do not fabricate.

## Product Principles
- Signal over noise: surface new releases fast, keep everything else quiet.
- Self-hosted trust: user owns data and credentials; be explicit about what leaves the instance.
- One feed, any source: platform differences stay behind the scenes.
- Works the same on phone and desktop.

## Accessibility & Inclusion
Target WCAG AA.
