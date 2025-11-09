#!/usr/bin/env sh

docker build -t gosqlapi-test-mysql -f ./test-dockers/Dockerfile.mysql .
docker build -t gosqlapi-test-mariadb -f ./test-dockers/Dockerfile.mariadb .
docker build -t gosqlapi-test-postgresql -f ./test-dockers/Dockerfile.postgresql .
docker build -t gosqlapi-test-mssql -f ./test-dockers/Dockerfile.mssql .

sleep 5

docker run --name gosqlapi-test-mysql -p 13306:3306 -d gosqlapi-test-mysql
docker run --name gosqlapi-test-mariadb -p 13307:3306 -d gosqlapi-test-mariadb
docker run --name gosqlapi-test-postgresql -p 15432:5432 -d gosqlapi-test-postgresql
docker run --name gosqlapi-test-mssql -p 11433:1433 -d gosqlapi-test-mssql

sleep 5

go test

sleep 5

docker stop gosqlapi-test-mysql
docker stop gosqlapi-test-mariadb
docker stop gosqlapi-test-postgresql
docker stop gosqlapi-test-mssql

sleep 5

docker rm -v gosqlapi-test-mysql
docker rm -v gosqlapi-test-mariadb
docker rm -v gosqlapi-test-postgresql
docker rm -v gosqlapi-test-mssql
