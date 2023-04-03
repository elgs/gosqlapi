#!/usr/bin/env bash

build="build"

rm -rf $build

declare -a branches=(
    "all"
    "mysql" 
    "postgres",
    "pgx"
    "sqlite"
    "sqlserver"
    "oracle"
)

declare -a oses=(
    "linux"
    "darwin"
    "windows"
)

declare -a arches=(
    "amd64"
    "arm64"
)

for branch in "${branches[@]}"
do
    git checkout "$branch"
    for os in "${oses[@]}"
    do
        for arch in "${arches[@]}"
        do
            echo "Building $os/$arch"
            output_name="$build/$os/$arch/$(basename $(pwd))-$branch-$os-$arch-$(git rev-parse --short HEAD)"
            if [ $os = "windows" ]; then
                output_name+='.exe'
            fi
            GOOS="$os" GOARCH="$arch" GOPROXY=direct go build -ldflags "-s -w -extldflags '-static'" -o "$output_name"
        done
    done
done

git checkout master