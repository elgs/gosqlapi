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

for i in "${branches[@]}"
do
    git checkout "$i"
    git pull origin "$i"
    git merge master --no-edit
    git push origin "$i"
done

git checkout master
