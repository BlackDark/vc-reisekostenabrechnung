#!/bin/sh
# Dispatch workflow_dispatch. release-please runs this after it creates or
# updates a release pull request (or publishes a tag). gh needs --repo: the
# historical step had no checkout, so gh died with "fatal: not a git repository"
# before it called the API.
#
# GITHUB_TOKEN can dispatch only when the repository allows Actions to create
# and approve pull requests. That 403 is a configuration warning, not a failed
# release-please run.
set -eu

workflow=${1:?workflow file}
ref=${2:?git ref}
repo=${GH_REPO:?GH_REPO}

echo "Dispatching ${workflow} on ${ref} in ${repo}"
set +e
out=$(gh workflow run "$workflow" --repo "$repo" --ref "$ref" 2>&1)
code=$?
set -e
printf '%s\n' "$out"
if [ "$code" -eq 0 ]; then
	exit 0
fi

case "$out" in
*"create and approve pull requests"*|*"Resource not accessible by integration"*|*"not permitted to"*)
	echo "::warning title=workflow dispatch skipped::${workflow} was not started on ${ref}. Settings → Actions → General → Allow GitHub Actions to create and approve pull requests is off, or the token is missing actions: write. ${out}"
	exit 0
	;;
esac

exit "$code"
