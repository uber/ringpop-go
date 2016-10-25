#!/bin/bash
#
# Same as `go get` but you can specify a git commit ref.
#
set -e

#
# Resolve the path to the Go workspace from GOPATH.
#
_go_workspace() {
	local gopath

	# If GOPATH has multiple paths separate by a colon, use the first one as
	# the Go workspace.
	gopath="${GOPATH%%:*}"

	echo "$gopath"
}

#
# List the Go package(s) in the current working directory, given the (optional)
# path spec.
#
_go_list() {
	local path=$1
	go list ./${path}
}

#
# Extract the version from the given string in the format "package@version"
#
_parse_version() {
	local package_spec="$1"
	if [ $(expr "$package_spec" : .*@) -ne 0 ]; then
		echo ${1##*@}
	fi
}

#
# Extract the base repo from the package spec/URL.
#
_parse_repo() {
	local package_spec=$1

	# Split by '/'
	IFS='/' read -ra path_components <<< "$package_spec"

	echo "${path_components[0]}/${path_components[1]}/${path_components[2]}"
}


#
# Extract the package path from the given package path.
#
# The path means the bit after the base (repo) name, for example given the
# following package name:
#
#   github.com/foo/mypackage/baz/.../
#
# The path would be:
#
#   /baz/.../
#
_parse_path() {
	local package_spec=$1
	local package_base=$(_parse_repo "$package_spec")

	path=${package_spec#$package_base}	# Strip base from front
	path=${path%%@*}					# Strip version from back

	echo $path
}

#
# Echos and runs the specified command.
#
run() {
    echo "+ $@" >&2
    "$@"
}

#
# Print usage text to stderr.
#
_usage() {
	{
		echo
		echo "go gets packages at specific versions"
		echo
		echo "Usage: $0 <package@version ...>"
		echo " e.g.: $0 github.com/foo/foo-package@31c913b github.com/foo/bar-package@3020345"
		echo
	} >&2
}

_main() {
	if [ "$#" -eq 0 ]; then
		_usage
		exit 99
	fi

	local package_repo
	local package_path
	local package_version

	local go_workspace="$(_go_workspace)"

	for package_spec in "$@"; do

		package_repo=$(_parse_repo "$package_spec")
		package_path=$(_parse_path "$package_spec")
		package_version=$(_parse_version "$package_spec")

		# echo package_repo: $package_repo
		# echo package_path: $package_path
		# echo package_version: $package_version
		# exit

		# Download package
		run go get -d "${package_repo}${package_path}"

		pushd "${go_workspace}/src/${package_repo}" >/dev/null

		if [ ! -z "$package_version" ]; then
			echo "# cd $PWD" >&2
			run git checkout -q $package_version
			# install dependencies for checked out version
			run go get ./...
		fi

		# Generate list of sub packages
		subpackages=$(_go_list "$package_path")

		popd >/dev/null

		# Build and install each package
		for subpackage in $subpackages; do
			pushd "${go_workspace}/src/${subpackage}" >/dev/null
			echo "# cd $PWD" >&2
			run go build
			run go install
			popd >/dev/null
		done
	done
}

_main "$@"
