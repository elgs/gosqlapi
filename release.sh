#!/usr/bin/env bash

declare -a branches=(
"all"
"mysql" 
"pgx"
"sqlite"
"sqlite3"
"sqlserver"
"oracle"
)

declare -a do_not_merge=(
    "go.mod"
    "go.sum"
)

for branch in "${branches[@]}"
do
    git checkout "$branch"
    git pull origin "$branch"
    git merge master --no-ff --no-commit
    for file in "${do_not_merge[@]}"
    do
        git reset HEAD -- "$file"
        git checkout -- "$file"
    done
    go get -u
    go mod tidy
    git commit -m "Merge branch 'master' into $branch"
    git push origin "$branch"
done

git checkout master
