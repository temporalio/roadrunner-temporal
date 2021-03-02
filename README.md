[![Build status](https://badge.buildkite.com/fc0e676d7bee1a159916af52ebdb541708d4b9f88b8a980f6b.svg?branch=master)](https://buildkite.com/temporal/roadrunner-temporal)
[![Linux](https://github.com/temporalio/roadrunner-temporal/workflows/Linux/badge.svg)](https://github.com/temporalio/roadrunner-temporal/actions)
[![Linters](https://github.com/temporalio/roadrunner-temporal/workflows/Linters/badge.svg)](https://github.com/temporalio/roadrunner-temporal/actions)
[![codecov](https://codecov.io/gh/temporalio/roadrunner-temporal/branch/master/graph/badge.svg?token=i3oU4IKmba)](https://codecov.io/gh/temporalio/roadrunner-temporal)
[![Discourse](https://img.shields.io/static/v1?label=Discourse&message=Get%20Help&color=informational)](https://community.temporal.io)

# Roadrunner Temporal
The repository contains a number of plugins which enable workflow and activity processing for PHP processes. The communication protocol,
supervisor, load-balancer is based on [RoadRunner PHP Application Server](https://roadrunner.dev).

## Installation
Temporal is official plugin of RoadRunner and available out of the box in [RoadRunner 2.0](https://github.com/spiral/roadrunner).

Read more about application server installation [here](https://roadrunner.dev/docs/intro-install).

To install PHP-SDK:

```bash
$ composer require temporal/sdk
```

Read how to configure your worker and init workflows [here](https://github.com/temporalio/sdk-php).

## Testing
To test integration make sure to install Golang and PHP 7.4+ at the same host. [Composer](https://getcomposer.org/) is required to manage the extension.

```bash
$ make install-dependencies
$ make start-docker-compose
$ make test
```

## License
[MIT License](https://github.com/temporalio/roadrunner-temporal/blob/master/LICENSE)