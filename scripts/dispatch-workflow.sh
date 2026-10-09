#!/bin/sh
# Dispatch workflow_dispatch.
#
# `gh workflow run` resolves the repository with git when --repo is omitted.
# The release-please job used to run that command without a checkout, so gh
# exited with "fatal: not a git repository" before it called the API.
# --repo skips that lookup. GH_REPO and GH_TOKEN come from the workflow.
set -eu

workflow=${1:?workflow file}
ref=${2:?git ref}
repo=${GH_REPO:?GH_REPO}

echo "Dispatching ${workflow} on ${ref} in ${repo}"
gh workflow run "$workflow" --repo "$repo" --ref "$ref"
