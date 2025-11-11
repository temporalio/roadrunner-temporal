## Acceptance Tests

This directory contains acceptance tests to verify the functionality of RoadRunner and PHP SDK integration with Temporal server.
The PHP SDK is included as a submodule in this repository.

### Setup

To run the PHP SDK, you need to install Composer dependencies:

```bash
cd tests/acceptance/php-sdk
composer update
```

Next, you need to download the Temporal Dev Server and Temporal Test Server.
This is done using the DLoad utility, which comes bundled with the PHP SDK.
DLoad automatically fetches the correct binary versions from the configuration, which is especially useful since different PHP SDK branches may require different Temporal server versions (including pre-release builds with experimental features).

Download the binaries with:

```bash
cd tests/acceptance/php-sdk
vendor/bin/dload get temporal temporal-tests-server
```

To build RoadRunner with the Temporal plugin, we also use DLoad. It fetches the plugin version numbers from build.roadrunner.dev, generates the Velox configuration, and runs Velox to build RoadRunner with the current codebase.

```bash
cd tests/acceptance
php-sdk/vendor/bin/dload build
```

### Running Tests

Navigate to the php-sdk directory and run the Acceptance or Functional tests using Composer:

```bash
cd tests/acceptance/php-sdk
composer test:acc
composer test:func
```