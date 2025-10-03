#!/usr/bin/env sh

# export test_all=1
# export mysql_url="test_user:TestPass123!@tcp(127.0.0.1:13306)/test_db"
# export mariadb_url="test_user:TestPass123!@tcp(127.0.0.1:13307)/test_db"
# export postgres_url="postgres://test_user:TestPass123!@localhost:15432/test_db?sslmode=disable"
# export pgx_url="postgres://test_user:TestPass123!@localhost:15432/test_db"
# export sqlserver_url="sqlserver://sa:TestPass123!@localhost:11433?database=test_db"

docker build -t gosqlapi-test-mysql -f ./test-dockers/Dockerfile.mysql .
docker build -t gosqlapi-test-mariadb -f ./test-dockers/Dockerfile.mariadb .
docker build -t gosqlapi-test-postgresql -f ./test-dockers/Dockerfile.postgresql .
docker build -t gosqlapi-test-mssql -f ./test-dockers/Dockerfile.mssql .

docker run --name gosqlapi-test-mysql -p 13306:3306 -d gosqlapi-test-mysql
docker run --name gosqlapi-test-mariadb -p 13307:3306 -d gosqlapi-test-mariadb
docker run --name gosqlapi-test-postgresql -p 15432:5432 -d gosqlapi-test-postgresql
docker run --name gosqlapi-test-mssql -p 11433:1433 -d gosqlapi-test-mssql

sleep 5

go test

docker stop gosqlapi-test-mysql
docker stop gosqlapi-test-mariadb
docker stop gosqlapi-test-postgresql
docker stop gosqlapi-test-mssql

sleep 5

docker rm -v gosqlapi-test-mysql
docker rm -v gosqlapi-test-mariadb
docker rm -v gosqlapi-test-postgresql
docker rm -v gosqlapi-test-mssql
