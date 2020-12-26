#!/bin/sh

z40=0000000000000000000000000000000000000000

while read -r _ local_sha _ remote_sha; do
	if [ "$local_sha" = $z40 ]; then
		# commit is deleted
		break
	fi

	# get range to check: all commits in case new branch or ones which starts from latest "remote"
	if [ "$remote_sha" = $z40 ]; then
		range="$local_sha"
	else
		range="$remote_sha..$local_sha"
	fi

    ### test if commits have "WIP" in the beggining
    wipHash=$(git rev-list -n 1 --grep '^WIP' "$range")
    if [ ! -z "$wipHash" ]; then
        echo "error: commits contain WIP messages, please rebase them before pushing, example:"
        echo "git stash"
        echo "git rebase -i HEAD~10"
        echo "git stash pop"
        exit 1
    fi

done

# exit 1