# gosqlapi

Turns any SQL database into a RESTful API. Currently supports MySQL, MariaDB, PostgreSQL, Oracle, Microsoft SQL Server and SQLite.

## Installation

```bash
$ go install github.com/elgs/gosqlapi@latest
```

If you run into any CGO related compilation issues because of `github.com/mattn/go-sqlite3` sqlite3 driver, you can try to install `gosqlapi` with the following command:

```bash
$ CGO_ENABLED=0 go install github.com/elgs/gosqlapi@latest
```

or on Windows:

```powershell
> cmd /C "set CGO_ENABLED=0&& go install github.com/elgs/gosqlapi@latest"
```

Please note that there must be no space between `CGO_ENABLED=0` and `&&`.

In this case, you will have to set database `type` as `sqlite` instead of `sqlite3` when you use SQLite3 database. For example:

```json
{
  "databases": {
    "test_db": {
      "type": "sqlite",
      "url": "./test.sqlite3"
    }
  }
}
```

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
      "type": "sqlite3",
      "url": "./test.sqlite3"
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
      "name": "test_table",
      "public_read": true,
      "public_write": true
    }
  }
}
```

`init.sql`:

```sql
drop TABLE IF EXISTS test_table;
create TABLE IF NOT EXISTS test_table(
    ID INTEGER NOT NULL PRIMARY KEY,
    NAME TEXT
);

insert INTO test_table (ID, NAME) VALUES (1, 'Alpha');
insert INTO test_table (ID, NAME) VALUES (2, 'Beta');
insert INTO test_table (ID, NAME) VALUES (3, 'Gamma');

-- @label: data
SELECT * FROM test_table WHERE ID > ?low? AND ID < ?high?;
```

### On the client side

#### Run a pre-defined script

```bash
$ curl -X EXEC 'http://localhost:8080/test_db/init/' \
  --header 'Content-Type: application/json' \
  --data-raw '{
  "low": 0,
  "high": 3
}'
{"data":[{"id":1,"name":"Alpha"},{"id":2,"name":"Beta"}]}
```

#### Get a recode

```bash
$ curl -X GET 'http://localhost:8080/test_db/test_table/1'
{"id":1,"name":"Alpha"}
```

#### Create a new recode

```bash
$ curl -X POST 'http://localhost:8080/test_db/test_table' \
  --header 'Content-Type: application/json' \
  --data-raw '{
  "id": 4,
  "name": "Gamma"
}'
{"last_insert_id":4,"rows_affected":1}
```

#### Update a recode

```bash
$ curl -X PATCH 'http://localhost:8080/test_db/test_table/4' \
  --header 'Content-Type: application/json' \
  --data-raw '{
  "name": "Omega"
}'
{"last_insert_id":4,"rows_affected":1}
```

#### Delete a recode

```bash
$ curl -X DELETE 'http://localhost:8080/test_db/test_table/4'
{"last_insert_id":4,"rows_affected":1}
```

#### Search for recodes

```bash
$ curl -X GET 'http://localhost:8080/test_db/test_table?name=Beta'
[{"id":2,"name":"Beta"}]
```

## Access Control

When a script has `public_exec` set to true, it can be executed by public users. When a table has `public_read` set to true, it can be read by public users. When a table has `public_write` set to true, it can be written by public users.

When a script or table is set to not be accessible by public users, an auth token is required to access the script or table. The client should send the auth token back to the server in the `Authorization` header. The server will verify the auth token and return an error if the auth token is invalid.

Auth tokens can be configured in `gosqlapi.json`:

```json
{
  "tokens": {
    "401d2fe0a18b26b4ce5f16c76cca6d484707f70a3a804d1c2f5e3fa1971d2fc0": [
      {
        "database": "test_db",
        "objects": ["test_table"],
        "read": true,
        "write": true
      },
      {
        "database": "test_db",
        "objects": ["init"],
        "exec": true
      }
    ]
  }
}
```

In the example above, the auth token is configured to allow the user to read and write `test_table` and execute `init` script in `test_db`.

## Pre-defined Scripts

There are a few things to note when defining a pre-defined script:

```sql
drop TABLE IF EXISTS test_table;
create TABLE IF NOT EXISTS test_table(
    ID INTEGER NOT NULL PRIMARY KEY,
    NAME TEXT
);

insert INTO test_table (ID, NAME) VALUES (1, 'Alpha');
insert INTO test_table (ID, NAME) VALUES (2, 'Beta');
insert INTO test_table (ID, NAME) VALUES (3, 'Gamma');

-- @label: data
SELECT * FROM test_table WHERE ID > ?low? AND ID < ?high?;
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
      "sql": "drop TABLE IF EXISTS test_table; create TABLE IF NOT EXISTS test_table( ID INTEGER NOT NULL PRIMARY KEY, NAME TEXT ); insert INTO test_table (ID, NAME) VALUES (1, 'Alpha'); insert INTO test_table (ID, NAME) VALUES (2, 'Beta'); insert INTO test_table (ID, NAME) VALUES (3, 'Gamma'); -- @label: data \n SELECT * FROM test_table WHERE ID > ?low? AND ID < ?high?;"
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
      "url": "sqlserver://user:pass@localhost:1433/test_db?param1=value&param2=value"
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
WorkingDirectory=/home/elgs/gosqlapi/
ExecStart=/home/elgs/go/bin/gosqlapi -c /home/elgs/gosqlapi/gosqlapi.json

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
