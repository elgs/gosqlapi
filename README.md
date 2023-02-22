# gosqlapi

Turns any SQL database into a RESTful API. Currently supports MySQL, MariaDB, PostgreSQL, and SQLite.

## Installation

```bash
$ go install github.com/elgs/gosqlapi@latest
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
