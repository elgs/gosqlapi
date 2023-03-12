# gosqlapi

Turns any SQL database into a RESTful API. Currently supports MySQL, MariaDB, PostgreSQL, Oracle, Microsoft SQL Server and SQLite.

The intention of `gosqlapi` is not to replace a full-fledged backend server, but to provide a quick and easy way to expose any SQL databases as a RESTful API without writing any server code, except for only SQL scripts.

## Installation

Install `gosqlapi` with one of the following commands, depending on the databases you want to use:

```bash
# Install gosqlapi for all databases, except mattn's sqlite3 because of CGO.
$ go install github.com/elgs/gosqlapi@all

# Install gosqlapi for MySQL and MariaDB - https://github.com/go-sql-driver/mysql
$ go install github.com/elgs/gosqlapi@mysql

# Install gosqlapi for PostgreSQL - https://github.com/jackc/pgx
$ go install github.com/elgs/gosqlapi@pgx

# Install gosqlapi for Oracle - https://github.com/sijms/go-ora
$ go install github.com/elgs/gosqlapi@oracle

# Install gosqlapi for Microsoft SQL Server - https://github.com/microsoft/go-mssqldb
$ go install github.com/elgs/gosqlapi@sqlserver

# Install gosqlapi for SQLite3 - https://github.com/mattn/go-sqlite3 (CGO required)
$ go install github.com/elgs/gosqlapi@sqlite3

# Install gosqlapi for SQLite - https://pkg.go.dev/modernc.org/sqlite
$ go install github.com/elgs/gosqlapi@sqlite

# Install gosqlapi for all databases from the latest commit. Things may break.
$ go install github.com/elgs/gosqlapi@latest
```

To remove `gosqlapi`:

```bash
$ go clean -i github.com/elgs/gosqlapi
```

### If installation fails because of CGO

If you run into any CGO related compilation issues because of `github.com/mattn/go-sqlite3` sqlite3 driver, you can try to install `gosqlapi` with the following command:

```bash
$ CGO_ENABLED=0 go install github.com/elgs/gosqlapi@all
```

or on Windows:

```powershell
> cmd /C "set CGO_ENABLED=0&& go install github.com/elgs/gosqlapi@all"
```

Please note that there must be no space between `CGO_ENABLED=0` and `&&`.

In this case, you will have to set database `type` as `sqlite` instead of `sqlite3` when you use SQLite3 database. For example:

```json
{
  "databases": {
    "test_db": {
      "type": "sqlite",
      "url": ":memory:"
    }
  }
}
```

### Download pre-built binaries

If you don't have Go installed, you can download pre-built binaries from the [releases page](https://goweb.az.ht/gosqlapi/build/).

## Usage

```bash
$ gosqlapi
```

or if you don't have `gosqlapi.json` in the current directory:

```bash
$ gosqlapi -c /path/to/gosqlapi.json
```

## Hello World

### On the server side

Prepare `gosqlapi.json` and `init.sql` in the current directory, and run `gosqlapi`:

`gosqlapi.json`:

```json
{
  "web": {
    "http_addr": "127.0.0.1:8080",
    "cors": true
  },
  "databases": {
    "test_db": {
      "type": "sqlite",
      "url": ":memory:"
    }
  },
  "scripts": {
    "init": {
      "database": "test_db",
      "path": "init.sql",
      "public_exec": true
    }
  },
  "tables": {
    "test_table": {
      "database": "test_db",
      "name": "TEST_TABLE",
      "public_read": true,
      "public_write": true
    }
  }
}
```

`init.sql`:

```sql
drop TABLE IF EXISTS TEST_TABLE;
create TABLE IF NOT EXISTS TEST_TABLE(
    ID INTEGER NOT NULL PRIMARY KEY,
    NAME VARCHAR(50)
);

insert INTO TEST_TABLE (ID, NAME) VALUES (1, 'Alpha');
insert INTO TEST_TABLE (ID, NAME) VALUES (2, 'Beta');
insert INTO TEST_TABLE (ID, NAME) VALUES (3, 'Gamma');

-- @label: data
SELECT * FROM TEST_TABLE WHERE ID > ?low? AND ID < ?high?;
```

### On the client side

#### Run a pre-defined SQL query

```bash
$ curl -X PATCH 'http://localhost:8080/test_db/init/' \
  --header 'Content-Type: application/json' \
  --data-raw '{
  "low": 0,
  "high": 3
}'
{"data":[{"id":1,"name":"Alpha"},{"id":2,"name":"Beta"}]}
```

#### Get a record

```bash
$ curl -X GET 'http://localhost:8080/test_db/test_table/1'
{"id":1,"name":"Alpha"}
```

#### Create a new record

```bash
$ curl -X POST 'http://localhost:8080/test_db/test_table' \
  --header 'Content-Type: application/json' \
  --data-raw '{
  "id": 4,
  "name": "Delta"
}'
{"last_insert_id":4,"rows_affected":1}
```

#### Update a record

```bash
$ curl -X PUT 'http://localhost:8080/test_db/test_table/4' \
  --header 'Content-Type: application/json' \
  --data-raw '{
  "name": "Omega"
}'
{"last_insert_id":4,"rows_affected":1}
```

#### Delete a record

```bash
$ curl -X DELETE 'http://localhost:8080/test_db/test_table/4'
{"last_insert_id":4,"rows_affected":1}
```

#### Search for records

```bash
$ curl -X GET 'http://localhost:8080/test_db/test_table?name=Beta'
[{"id":2,"name":"Beta"}]
```

#### Search for records with .page_size, .offset and .order_by

```bash
$ curl --request GET \
  --url 'http://localhost:8080/test_db/test_table?.page_size=2&.offset=1'
{
  "data": [
    {
      "id": 2,
      "name": "Beta"
    },
    {
      "id": 3,
      "name": "Gamma"
    }
  ],
  "offset": 1,
  "page_size": 2,
  "total": 3
}
```

You can use the following parameters:

- `.page_size`: maximum number of records returned
- `.offset`: offset the number of records returned
- `.order_by`: order the records returned

You can give a table a default page_size and order_by by setting `page_size` and `order_by` in `gosqlapi.json`:

```json
{
  "tables": {
    "test_table": {
      "database": "test_db",
      "name": "TEST_TABLE",
      "public_read": true,
      "public_write": true,
      "page_size": 10,
      "order_by": "name asc, id desc"
    }
  }
}
```

## Access Control

When a script has `public_exec` set to true, it can be executed by public users. When a table has `public_read` set to true, it can be read by public users. When a table has `public_write` set to true, it can be written by public users.

When a script or table is set to not be accessible by public users, an auth token is required to access the script or table. The client should send the auth token back to the server in the `Authorization` header. The server will verify the auth token and return an error if the auth token is invalid.

### Simple Tokens

Simple tokens are configured in `gosqlapi.json`:

```json
{
  "tokens": {
    "401d2fe0a18b26b4ce5f16c76cca6d484707f70a3a804d1c2f5e3fa1971d2fc0": [
      {
        "target_database": "test_db",
        "target_objects": ["test_table"],
        "read_private": true,
        "write_private": true
      },
      {
        "target_database": "test_db",
        "target_objects": ["init"],
        "exec_private": true
      }
    ]
  }
}
```

In the example above, the auth token is configured to allow users to read and write `test_table` and execute `init` script in `test_db`.

### Managed Tokens

#### Token Table

Managed tokens are stored in the database. The table and database that will store managed tokens are configured as `managed_tokens` in `gosqlapi.json`.

```json
{
  "managed_tokens": {
    "database": "test_db",
    "table_name": "TOKENS"
  }
}
```

The table that stores managed tokens should have the following schema:

```sql
CREATE TABLE IF NOT EXISTS `TOKENS` (
  `ID` CHAR(36) NOT NULL,
  `USER_ID` CHAR(36) NOT NULL,
  `TOKEN` VARCHAR(255) NOT NULL,              -- required, auth token
  `TARGET_DATABASE` VARCHAR(255) NOT NULL,    -- required, target database
  `TARGET_OBJECTS` TEXT NOT NULL,             -- required, target objects, separated by whitespace
  `READ_PRIVATE` INT NOT NULL DEFAULT 0 ,     -- required, 1: read, 0: no read
  `WRITE_PRIVATE` INT NOT NULL DEFAULT 0 ,    -- required, 1: write, 0: no write
  `EXEC_PRIVATE` INT NOT NULL DEFAULT 0 ,     -- required, 1: exec, 0: no exec
  CONSTRAINT `PRIMARY` PRIMARY KEY (`ID`)
);
create INDEX TOKEN_INDEX ON TOKENS (TOKEN);
```

Please feel free to change the ID to a different type, such as `INT`, or add more columns to the table. The only requirement is that the table should have the required columns listed above. Also consider adding an index to the `TOKEN` column.

When `managed_tokens` is configured in `gosqlapi.json`, the `tokens` in `gosqlapi.json` will be ignored.

If you already have a table that stores managed tokens, you can map the fields in that token table as follows:

```json
{
  "managed_tokens": {
    "database": "test_db",
    "table_name": "TOKENS",
    "token": "AUTH_TOKEN",
    "target_database": "TARGET_DATABASE",
    "target_objects": "TARGET_OBJECTS",
    "read_private": "READ_PRIVATE",
    "write_private": "WRITE_PRIVATE",
    "exec_private": "EXEC_PRIVATE"
  }
}
```

For example, if your token table has the field `AUTH_TOKEN` instead of `TOKEN`, you can use the configuration above to map the field `AUTH_TOKEN` to `TOKEN`.

#### Token Query

Instead of specifying the `table_name`, you can use a `query` in the config. The `query` should return the same columns as the token table.

```json
{
  "managed_tokens": {
    "database": "test_db",
    "query": "SELECT TARGET_DATABASE AS target_database, TARGET_OBJECTS AS target_objects, READ_PRIVATE AS read_private, WRITE_PRIVATE AS write_private, EXEC_PRIVATE AS exec_private FROM TOKENS WHERE TOKEN=?token?"
  }
}
```

The placeholder will be replaced with the auth token. If the `query` is getting too long, you can use a separate file to store the query.

```json
{
  "managed_tokens": {
    "database": "test_db",
    "query_path": "token_query.sql"
  }
}
```

## Pre-defined SQL Queries

There are a few things to note when defining a pre-defined SQL query in a script:

```sql
drop TABLE IF EXISTS TEST_TABLE;
create TABLE IF NOT EXISTS TEST_TABLE(
    ID INTEGER NOT NULL PRIMARY KEY,
    NAME TEXT
);

insert INTO TEST_TABLE (ID, NAME) VALUES (1, 'Alpha');
insert INTO TEST_TABLE (ID, NAME) VALUES (2, 'Beta');
insert INTO TEST_TABLE (ID, NAME) VALUES (3, 'Gamma');

-- @label: data
SELECT * FROM TEST_TABLE WHERE ID > ?low? AND ID < ?high?;
```

1. You can define multiple SQL statements in a single script. The statements will be executed in the order they appear in the script. The script will be executed in a transaction. If any statement fails, the transaction will be rolled back, and if all statements succeed, the transaction will be committed. Statements in the script are separated by `;`.
2. The results of the statements that start with an uppercase letter will be returned to the client. The results of the statements that start with a lowercase letter will not be returned to the client.
3. You can label a statement with `-- @label: label_name`. The `label_name` will be the key of the result in the returned JSON object.
4. You can use `?param_name?` to define a parameter. The `param_name` will be the key of the parameter in the JSON object sent to the server.

### Inline scripts

You have the option to define a script inline in the `gosqlapi.json` file. This is useful when you want to define a script that is short or you don't want to create a separate file for the script. The script can be defined in the `gosqlapi.json` file as follows:

```json
{
  "scripts": {
    "init": {
      "database": "test_db",
      "sql": "drop TABLE IF EXISTS TEST_TABLE; create TABLE IF NOT EXISTS TEST_TABLE( ID INTEGER NOT NULL PRIMARY KEY, NAME TEXT ); insert INTO TEST_TABLE (ID, NAME) VALUES (1, 'Alpha'); insert INTO TEST_TABLE (ID, NAME) VALUES (2, 'Beta'); insert INTO TEST_TABLE (ID, NAME) VALUES (3, 'Gamma'); -- @label: data \n SELECT * FROM TEST_TABLE WHERE ID > ?low? AND ID < ?high?;"
    }
  }
}
```

When both `sql` and `path` are defined, `path` will be used, and `sql` will be ignored.

### Edit scripts in dev mode

When the server is running in dev mode, the server will not cache the scripts and will reload the scripts every time a request is made. This is useful when you are editing the scripts so that you don't have to restart the server every time you make a change. To run the server in dev mode, set the `env` environment variable to `dev` when starting the server:

```bash
$ env=dev gosqlapi
```

`dev` mode is only effective for scripts defined in `gosqlapi.json` by `path`. For scripts defined in `gosqlapi.json` by `sql`, `dev` mode will not be effective.

Do not use dev mode in production, as it will read the scripts from the disk every time a request is made.

## Request Metadata in Pre-defined SQL Queries

You can access the request metadata in pre-defined SQL queries. The request metadata includes the request method, the request path, the request query string, and the request headers. The request metadata can be accessed in the pre-defined SQL queries as follows:

```sql
SELECT
!remote_addr! as REMOTE_ADDRESS,
!host! as HOST,
!method! as METHOD,
!path! as PATH,
!query! as QUERY,
!user_agent! as USER_AGENT,
!referer! as REFERER,
!accept! as ACCEPT,
!AUThorization! as AUTHORIZATION;
```

The request metadata parameters are case-insensitive. The request metadata parameters are surrounded by `!` characters.

## Database Configuration

### SQLite3

```json
{
  "databases": {
    "test_db": {
      "type": "sqlite3",
      "url": "./test_db.sqlite3"
    }
  }
}
```

https://github.com/mattn/go-sqlite3

or

```json
{
  "databases": {
    "test_db": {
      "type": "sqlite",
      "url": "./test_db.sqlite3"
    }
  }
}
```

https://pkg.go.dev/modernc.org/sqlite

### MySQL and MariaDB

```json
{
  "databases": {
    "test_db": {
      "type": "mysql",
      "url": "user:pass@tcp(localhost:3306)/test_db"
    }
  }
}
```

https://github.com/go-sql-driver/mysql

### PostgreSQL

```json
{
  "databases": {
    "piq": {
      "type": "pgx",
      "url": "postgres://user:pass@localhost:5432/test_db"
    }
  }
}
```

https://github.com/jackc/pgx

### Microsoft SQL Server

```json
{
  "databases": {
    "test_db": {
      "type": "sqlserver",
      "url": "sqlserver://user:pass@localhost:1433?database=test_db"
    }
  }
}
```

https://github.com/microsoft/go-mssqldb

### Oracle

```json
{
  "databases": {
    "test_db": {
      "type": "oracle",
      "url": "oracle://user:pass@localhost:1521/test_db"
    }
  }
}
```

### Oracle Cloud TLS

```json
{
  "databases": {
    "test_db": {
      "type": "oracle",
      "url": "oracle://user:pass@:0/?SSL VERIFY=FALSE&connStr=(description=(retry_count=20)(retry_delay=3)(address=(protocol=tcps)(port=1521)(host=host))(connect_data=(service_name=service_name))(security=(ssl_server_dn_match=yes)))"
    }
  }
}
```

https://github.com/sijms/go-ora

## HTTPS

Here is an example of how to configure HTTPS:

```json
{
  "web": {
    "http_addr": "127.0.0.1:8080",
    "https_addr": "127.0.0.1:8443",
    "cert_file": "/path/to/cert.pem",
    "key_file": "/path/to/key.pem",
    "cors": true
  }
}
```

## Auto start with systemd

Create service unit file `/etc/systemd/system/gosqlapi.service` with the following content:

```
[Unit]
After=network.target

[Service]
WorkingDirectory=/home/user/gosqlapi/
ExecStart=/home/user/go/bin/gosqlapi -c /home/user/gosqlapi/gosqlapi.json

[Install]
WantedBy=default.target
```

Enable the service:

```
$ sudo systemctl enable gosqlapi
```

Remove the service:

```
$ sudo systemctl disable gosqlapi
```

Start the service

```
$ sudo systemctl start gosqlapi
```

Stop the service

```
$ sudo systemctl stop gosqlapi
```

Check service status

```sh
$ sudo systemctl status gosqlapi
```

## License

MIT License

Copyright (c) 2023 Qian Chen

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
