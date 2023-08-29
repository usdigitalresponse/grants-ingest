#! /bin/bash

# Defaults
next_version_release_year=$(TZ='UTC' date '+%Y')
next_version_release_number=1

# Ensure tag history is available
git fetch --prune --unshallow

tag=$(git describe --tags --match='release/[0-9][0-9][0-9][0-9].[0-9]*' refs/heads/main)
regex='release\/[0-9]{4}\.([0-9]+)'
if [[ $tag =~ $regex ]]; then
    echo "Found tag for previous release: $tag"
    prev_version_release_number="${BASH_REMATCH[1]}"
    echo "Previous version number: $prev_version_release_number"
    ((next_version_release_number=prev_version_release_number+1))
else
    echo "Could not locate a previous release version" >&2
fi

next_version="$next_version_release_year.$next_version_release_number"
echo "Next version: $next_version"
echo "result=$next_version" >> $GITHUB_OUTPUT
