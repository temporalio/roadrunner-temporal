## Acceptance Tests

This directory contains acceptance tests to verify the functionality of RoadRunner and PHP SDK integration with Temporal server.
The PHP SDK is included as a submodule in this repository.

### Setup

First, setup `sdk-php` Git submodule:

```bash
git submodule update --remote
```

Now `tests/acceptance/php-sdk` contains PHP SDK source code.

Next, to run the PHP SDK, you need to install PHP dependencies:

```bash
cd tests/acceptance/php-sdk
composer install
```

Next, you need to download the Temporal Dev Server and Temporal Test Server.
This is done using the DLoad utility, which comes bundled with the PHP SDK.
DLoad automatically fetches the correct binary versions from the configuration, which is especially useful since different PHP SDK branches may require different Temporal server versions (including pre-release builds with experimental features).

Download the binaries with the following command: (from `tests/acceptance/php-sdk`)

```bash
composer get:binaries
```

The `composer get:binaries` command also downloads a released `rr` binary. Replace it with a RoadRunner `master` build that uses the current plugin code. Run these commands from the repository root:

```bash
git clone https://github.com/roadrunner-server/roadrunner.git
cd roadrunner
go mod edit -replace github.com/temporalio/roadrunner-temporal/v6=../
GOWORK=off go mod tidy
GOWORK=off CGO_ENABLED=0 go build -o ../tests/acceptance/php-sdk/rr ./cmd/rr
```

### Running Tests

Navigate to the php-sdk directory and run the Acceptance or Functional tests using Composer:

```bash
cd ../tests/acceptance/php-sdk
ROADRUNNER_BINARY='./rr' composer test:accept-fast
ROADRUNNER_BINARY='./rr' composer test:accept-slow
ROADRUNNER_BINARY='./rr' composer test:func
```