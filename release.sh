#!/usr/bin/env bash

declare -a branches=(
    "all"
    "mysql" 
    "postgres"
    "pgx"
    "sqlite"
    "sqlite3"
    "sqlserver"
    "oracle"
)

declare -a do_not_merge=(
    "go.mod"
    "go.sum"
    "drivers.go"
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
    ./update.sh
    git commit -am "Merge branch 'master' into $branch"
    git push origin "$branch"
done

go mod tidy
git checkout master
