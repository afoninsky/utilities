#!/bin/sh
### aliases for semantic commits
# usage: source aliases.sh

_gsemantic() {
    args=( "$@" )

    type="${args[1]}"
    scope=""
    message=""
    commit=""

    if [ ${args[2]:0:1} = "+" ]; then
        scope="${args[2]:1}"
        message="${args[@]:2}"
        commit="${type}(${scope}): ${message}"
    else
        message="${args[@]:1}"
        commit="${type}: ${message}"
    fi

    if [[ -z "$message" ]]; then
        echo "Invalid commit message, supported formats:"
        echo "\"<command> message\""
        echo "\"<command> +scope message\""
        echo "commands: gchore, gtest, gperf, gdoc, gref, gfix, gfeat"
    else
        # TODO: fix reset wip, reset till latest origin only
        # _resetWIP
        git add -A
        git commit -m "${commit}"
    fi
}

# TODO: stop when already pushed messages reached
_resetWIP() {
    latestSHA=$(git rev-parse HEAD)
    remoteSHA=$(git rev-parse origin/master 2> /dev/null)
    if [ "$?" -ne "0" ]; then
        range="$latestSHA"
    else
        range="$remoteSHA..$latestSHA"
    fi


    # iterates thru list of commits for given range resetting commits containing WIP
    # makes soft-reset if "WIP" commit found
    while read -r line; do
        case $line in
        *WIP*)
            parenthash=${line%% *}
            git reset --soft $parenthash
            ;;
        *)
            break
            ;;
        esac
    done <<< $(git log --boundary --pretty='format:%P %s' "$range")

}


gchore() {
    _gsemantic chore "$@"
}

gtest() {
    _gsemantic test "$@"
}

gperf() {
    _gsemantic perf "$@"
}

gdoc() {
    _gsemantic docs "$@"
}

gref() {
    _gsemantic refactor "$@"
}

gfix() {
    _gsemantic fix "$@"
}

gfeat() {
    _gsemantic feat "$@"
}

unalias gwip 2> /dev/null
gwip() {
    git add -A
    git commit -m "WIP: temporary commit [skip CI]"
}

# adds untracked changes to last commit
gadd() {
    git add -A
    git commit --amend --no-edit
}