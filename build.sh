#!/bin/bash


buildah bud --build-arg GIT_SHA="$(git rev-parse --short HEAD)" -f Dockerfile -t registry.whiskeyonthe.rocks/newreleases/newreleases:dev
buildah push registry.whiskeyonthe.rocks/newreleases/newreleases:dev

date
sleep 10
ssh 10.10.10.9 "myk redeploy newreleases"


