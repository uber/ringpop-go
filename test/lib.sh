# Common functions for test code

declare project_root="${0%/*}/.."
declare ringpop_common_dir="${0%/*}/ringpop-common"
declare ringpop_common_branch="feature/labels"

#
# Clones or updates the ringpop-common repository.
# Runs `npm install` in ringpop-common/$1.
#
fetch-ringpop-common() {
    if [ ! -e "$ringpop_common_dir" ]; then
        run git clone --depth=1 https://github.com/uber/ringpop-common.git "$ringpop_common_dir" --branch "$ringpop_common_branch"
    fi

    run cd "$ringpop_common_dir"
    #run git checkout latest version of $ringpop_common_branch
    run git fetch origin "$ringpop_common_branch"
    run git checkout "FETCH_HEAD"

    run cd - >/dev/null

    run cd "${ringpop_common_dir}/$1"
    if [[ ! -d "node_modules" && ! -d "../node_modules" ]]; then
        run npm install #>/dev/null
    fi
    run cd - >/dev/null
}

#
# Copy stdin to stdout but prefix each line with the specified string.
#
prefix() {
    local _prefix=

    [ -n "$1" ] && _prefix="[$1] "
    while IFS= read -r -t 30 line; do
        echo "${_prefix}${line}"
    done
}

#
# Echos and runs the specified command.
#
run() {
    echo "+ $@" >&2
    "$@"
}
