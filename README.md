# CockroachDB s3Checker

`s3Checker` is designed to ensure that nodes have sufficient access
to s3 buckets for CockroachDB bulk operations, including backup, restore, import, export
and enterprise changefeeds (when s3 storage is used). 
These requirements are outline in the 
[official documentation](https://www.cockroachlabs.com/docs/stable/use-cloud-storage-for-bulk-operations.html#storage-permissions).

`s3checker` uses the [AWS SDK for Go library - version1](https://docs.aws.amazon.com/sdk-for-go/index.html) by default, which
is the same library that CockroachDB uses (at least as of the current version 22.1). This makes it easier to
troubleshoot s3 access issues instead of attempting to run the various bulk operations in CockroachDB. It can also be
run with version 2 of the SDK.

It also outputs environment variables that use the prefix `AWS_` or include `proxy` or `PROXY`.

## Usage

`s3checker` can be executed with `implicit` or `explicit` authentication when creating a session.
The [AWS SDK for Go Documentation](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html)
details how sessions are created using both approaches. `implicit` authentication attempts to load credentials
from the local environment or via EC2 instance metadata when using IAM roles for EC2. When using `explicit` authentication,
`--key-id` and `--access-key` must be included. If temporary credentials have been generated, e.g., via STS,
the `session-token` must also be included.

The `--bucket` flag is required.

The `--region` flag is typically required if running from an EC2 instance that is in a different region than the s3 bucket.

By default, `s3checker` runs with the AWS SDK for Go version 1, which is currently used in CockroachDB. To run with SDK version 2,
use the `--sdk-version 2` flag.

```
Usage:
  s3checker [flags]

Flags:
      --access-key string      AWS secret access key, when using explicit auth
      --auth string            Auth type: implicit or explicit (default "implicit")
      --bucket string          S3 bucket
      --debug                  Include debug output for request errors
  -h, --help                   help for s3checker
      --key-id string          AWS access key ID, when using explicit auth
      --region string          AWS region, optional
      --sdk-version int        AWS SDK version, 1 or 2 (default 1)
      --session-token string   AWS session token, when using explicit auth and STS temporary credentials
  -t, --toggle                 Help message for toggle
```

## Examples

Implicit auth (default):

```
s3checker --bucket jon-test
```

Implicit auth w/ region:

```
s3checker --bucket jon-test --region us-east-1
```

Explicit auth:

```
s3checker --bucket jon-test --auth explicit --key-id XX123 --access-key ABC123
```

Explicit auth w/ session token (for temporary credentials):

```
s3checker --bucket jon-test --auth explicit --key-id XX123 --access-key ABC123 --session-token MNP1234
```

## Output

When `s3checker` runs it outputs the caller identity, result of important s3 operations and what bulk operations
are supported with the supported operations. If any errors, occur, those will also be output.

For example, the following output indicates that all bulk operations are supported:

```
Environment variables that contain 'proxy' or 'PROXY':
  myproxy=test

Environment variables that are prefixed with 'AWS_':
  AWS_BLAH=test

Caller identity:
{
  Account: "541263489771",
  Arn: "arn:aws:iam::541263421232:user/jon",
  UserId: "AIDAX4BORNLV2PXXXXXXXX"
}

S3 Operations:
list objects -- successful
put object -- successful
get object -- successful

Access sufficient for the following CockroachDB capabilities:
Backup -- sufficient
Restore -- sufficient
Import -- sufficient
Export -- sufficient
Enterprise Changefeeds -- sufficient

```

## Run from source code

To run from source code, check out the repository and run:

```
go run main.go --bucket jon-test
```

## Build for distribution

Build on platform for the same platform (e.g., linux for linux):

```
go build -o bin/s3checker
```

On mac, for linux:

```
GOOS=linux GOARCH=amd64 go build -o bin/s3checker
```

## Get Help

Reach out to jon@cockroachlabs.com with questions.