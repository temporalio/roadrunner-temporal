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

To build RoadRunner with the Temporal plugin, we also use DLoad. 
It fetches the plugin version numbers from build.roadrunner.dev, generates the Velox configuration, and runs Velox to build RoadRunner with the current codebase.

Run the following command to build RoadRunner: (from `tests/acceptance`)

> **Note:** Before building, you must generate a GitHub Personal Access Token at https://github.com/settings/personal-access-tokens and use it as the `GITHUB_TOKEN` environment variable.

```bash
cd ..
CGO_ENABLED=0 GITHUB_TOKEN=YOUR_TOKEN_HER php-sdk/vendor/bin/dload build
```

### Running Tests

Navigate to the php-sdk directory and run the Acceptance or Functional tests using Composer: (from `tests/acceptance/php-sdk`)

```bash
cd php-sdk
ROADRUNNER_BINARY='./rr' composer test:accept-fast
ROADRUNNER_BINARY='./rr' composer test:accept-slow
ROADRUNNER_BINARY='./rr' composer test:func
```