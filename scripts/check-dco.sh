#!/usr/bin/env sh
set -eu

range="${1:-${DCO_RANGE:-}}"

if [ -z "$range" ]; then
	base="${DCO_BASE_REF:-}"
	head="${DCO_HEAD_REF:-HEAD}"
	if [ -n "$base" ]; then
		range="$base..$head"
	elif [ -n "${GITHUB_BASE_REF:-}" ]; then
		range="origin/${GITHUB_BASE_REF}..HEAD"
	else
		upstream="$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"
		if [ -n "$upstream" ]; then
			range="$upstream..HEAD"
		else
			range="HEAD"
		fi
	fi
fi

commits="$(git log --format=%H "$range")"

if [ -z "$commits" ]; then
	echo "No commits to check for DCO sign-off."
	exit 0
fi

failed=0

for commit in $commits; do
	short="$(git rev-parse --short "$commit")"
	subject="$(git log -1 --format=%s "$commit")"
	message="$(git log -1 --format=%B "$commit")"

	if printf '%s\n' "$message" | git interpret-trailers --parse | grep -Eiq '^Signed-off-by: .+ <[^<>@[:space:]]+@[^<>[:space:]]+\.[^<>[:space:]]+>$'; then
		echo "ok: $short $subject"
	else
		echo "::error title=Missing DCO sign-off::$short $subject"
		failed=1
	fi
done

if [ "$failed" -ne 0 ]; then
	cat <<'MSG'

Every commit in this change must include a DCO sign-off trailer:

    Signed-off-by: Your Name <you@example.com>

Use `git commit -s` for new commits. To repair an existing local branch, run:

    git rebase --signoff <base-branch>

MSG
	exit 1
fi
