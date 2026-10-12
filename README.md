<img src="https://github.com/homedepot/go-clouddriver/blob/media/clouddriver.png" width="125" align="left">

# Go Clouddriver

Go Clouddriver is a rewrite of Spinnaker's [Clouddriver](https://github.com/spinnaker/clouddriver) microservice. It has an observed 95%+ decrease in CPU and memory load for Kubernetes operations.

Go Clouddriver brings many features to the table which allow it to perform better than Clouddriver OSS for production loads:

- it does not rely on `kubectl` and instead interfaces directly with the Kubernetes API for all operations
- it utilizes an in-memory cache store for Kubernetes API discovery
- it stores Kubernetes providers in a database, fronted by a simple CRUD API
- it removes over-complicated strategies such as [Cache All The Stuff](https://github.com/spinnaker/clouddriver/tree/master/cats), instead making live calls for all operations

Go Clouddriver is _not_ an API complete implementation of Clouddriver OSS and only handles Kubernetes providers and operations. It is meant to be run in tandem with Clouddriver OSS. Visit [the wiki](https://github.com/homedepot/go-clouddriver/wiki) for feature support and intallation instructions.

## Getting Started

### Prerequisites

- [Go](https://go.dev/doc/install) 1.22 or newer
- [golangci-lint](https://golangci-lint.run/welcome/install/) v1.58 (the version CI uses), needed by `make run` and `make lint`
- Ginkgo, the test runner. Install it from the root directory with

```bash
make tools
```

`make tools` installs `ginkgo` into `$(go env GOPATH)/bin`, so make sure that directory is on your `PATH`.

### Testing

Run from the root directory

```bash
make test
```

`make test` points `GOOGLE_APPLICATION_CREDENTIALS` at the fake key file in `test/credentials` so the artifact tests can run without real Google credentials.

### Building

Run from the root directory

```bash
make build
```

### Running Locally

1. Go Clouddriver generates its access tokens using [Arcade](https://github.com/homedepot/arcade) as a sidecar, so a working instance of Arcade will need to be running locally in order for Go Clouddriver to talk to Kubernetes clusters.

2. Export the Arcade API key (the same one you set up in step 1).

```bash
export ARCADE_API_KEY=test
```

3. Run Go Clouddriver. `make run` lints, builds and tests before starting the server.

```bash
make run
```

By default Go Clouddriver creates a local SQLite database, `clouddriver.db`, in the directory it is started from. No setup is needed.

To use MySQL instead, set `DB_HOST`, `DB_NAME`, `DB_USER` and `DB_PASS` (all four are required). An empty database is enough. Go Clouddriver creates its tables on start-up, and creates them as `utf8mb3`, so a MySQL 8 database with the default `utf8mb4` character set works. For example, with Docker:

```bash
docker run -d --name clouddriver-mysql -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=password -e MYSQL_DATABASE=clouddriver mysql:8.0

export DB_HOST=127.0.0.1:3306 DB_NAME=clouddriver DB_USER=root DB_PASS=password
make run
```

Check that the server is up:

```bash
curl localhost:7002/health
```

4. Create your first Kubernetes provider! Go Clouddriver runs on port 7002, so you'll make a POST to `localhost:7002/v1/kubernetes/providers`.

```bash
curl -XPOST localhost:7002/v1/kubernetes/providers -d '{
  "name": "test-provider",
  "host": "https://test-host",
  "caData": "test",
  "permissions": {
    "read": [
      "test-group"
    ],
    "write": [
      "test-group"
    ]
  }
}' | jq
```

And you should see the response

```json
{
  "name": "test-provider",
  "host": "https://test-host",
  "caData": "test",
  "permissions": {
    "read": ["test-group"],
    "write": ["test-group"]
  }
}
```

Running the command again will return a `409 Conflict` unless you change the name of the provider.

5. List your providers by calling the `/credentials` endpoint.

```bash
curl localhost:7002/credentials | jq
```

### Configuration

| Environment Variable               |                           Description                            |                                                         Notes | Default Value |
| ---------------------------------- | :--------------------------------------------------------------: | ------------------------------------------------------------: | ------------: |
| `ARCADE_API_KEY`                   | Needed to talk to [Arcade](https://github.com/billiford/arcade). |                                 Required for most operations. |               |
| `ARTIFACTS_CREDENTIALS_CONFIG_DIR` |         Sets the directory for artifacts configuration.          | Optional. Leave unset to use OSS Clouddriver's Artifacts API. |               |
| `KUBERNETES_USE_DISK_CACHE`        |  Stores Kubernetes API discovery on disk instead of in-memory.   |                                                               |       `false` |
| `DB_HOST`                          |                Used to connect to MySQL database.                |             If not set will default to local SQLite database. |               |
| `DB_NAME`                          |                Used to connect to MySQL database.                |             If not set will default to local SQLite database. |               |
| `DB_PASS`                          |                Used to connect to MySQL database.                |             If not set will default to local SQLite database. |               |
| `DB_USER`                          |                Used to connect to MySQL database.                |             If not set will default to local SQLite database. |               |
| `VERBOSE_REQUEST_LOGGING`          |              Logs all incoming request information.              |            Should only be used in non-production for testing. |       `false` |

### MySQL Indexes and Cleanup

Go Clouddriver stores all deployed resource requests in its `kubernetes_resources` table, which needs to be cleaned up periodically. It also requires a few indexes
to work efficiently and properly for continued deployments over long periods of time. These are defined by default when
starting the application.

#### Indexes

First, an index to help the Applications API remain efficient.

```sql
CREATE INDEX account_name_kind_name_spinnaker_app_idx ON kubernetes_resources(account_name, kind, name, spinnaker_app);
```

Next, an index to assist in pulling "cluster" kinds from the `kubernetes_resources` table.

```sql
CREATE INDEX kind_idx ON kubernetes_resources(kind);
```

Next, an index to assist the Task API.

```sql
CREATE INDEX task_id_idx ON kubernetes_resources(task_id);
```

Finally, a couple of indexes on the provider read/write permissions tables to help the Credentials API and any queries to select providers from the database.

```sql
CREATE INDEX provider_read_permissions_account_name_idx ON provider_read_permissions(account_name);
CREATE INDEX provider_write_permissions_account_name_idx ON provider_write_permissions(account_name);
```

Older versions named both of these indexes `account_name_idx`. On an existing MySQL database, Go Clouddriver creates the new indexes on start-up and leaves the old ones in place. Once the new indexes exist, you can drop the old ones:

```sql
DROP INDEX account_name_idx ON provider_read_permissions;
DROP INDEX account_name_idx ON provider_write_permissions;
```

#### SQL Cleanup

The `kubernetes_resources` tables is incredibly important for storing the most-recent state of a cluster for the Kubernetes kinds we care about.

It is recommended to run the following cleanup statements _daily_ to maintain a healthy state of onboarded Kubernetes clusters.

```sql
DELETE FROM kubernetes_resources where cluster = '' and timestamp < (NOW() - INTERVAL 6 HOUR);

DELETE t1 FROM kubernetes_resources t1
INNER JOIN kubernetes_resources t2
WHERE
    t1.id < t2.id AND
    t1.account_name = t2.account_name AND
    t1.api_group = t2.api_group AND
    t1.name = t2.name AND
    t1.namespace = t2.namespace AND
    t1.resource = t2.resource AND
    t1.version = t2.version AND
    t1.kind = t2.kind AND
    t1.cluster = t2.cluster AND
    t1.timestamp < (NOW() - INTERVAL 6 HOUR);
```
