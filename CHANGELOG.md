CHANGELOG
=========

v1.0.9 (--.08.2021)
-------------------

## 👀 New:

- ✏️ Expose all temporal metrics. New `metrics` options in the configuration to set up prometheus metrics.

## 🩹 Fixes:

- 🐛 Fix invalid child workflow ID error propagation to PHP worker. [BUG](https://github.com/temporalio/sdk-php/issues/113)


## 🧹 Updates:

- 🤖 Update temporal go-sdk to `v1.9.0` [Release](https://github.com/temporalio/sdk-go/releases/tag/v1.9.0).
- 📦 Update RR to 2.4.0

v1.0.8 (30.06.2021)
-------------------

## 🩹 Fixes:

- 🐛 Fix increase workflow 1-worker pool allocate timeout to 10 days. [BUG](https://github.com/temporalio/roadrunner-temporal/pull/74)

## 🧹 Updates:

- 📦 Bump RR to 2.3.1

v1.0.6 (12.05.2021)
-------------------

## 🧹 Updates:

- 📦 Bump RR to 2.2.0

v1.0.5 (29.04.2021)
-------------------

## 🧹 Updates:

- 📦 Bump RR to 2.1.1

v1.0.4 (27.04.2021)
-------------------

## 🧹 Updates:

- 🤖 Update `informer` interface.
- 📦 Bump RR to 2.1.0

v1.0.3 (06.04.2021)
-------------------

## 🩹 Fixes:

- 🐛 Fix bug with the worker which does not follow common grace period.

## 🧹 Updates:

- 📦 Bump RR to 2.0.4

v1.0.2 (09.03.2021)
-------------------

## 🧹 Updates:

- 📦 Bump RR to 2.0.2

v1.0.1 (09.03.2021)
-------------------
-🪛 Use `runCommand` instead of `pushCommand` because of phantom unexpected responses in the workflow process `Close`
method.

- 📦 Bump Endure container to v1.0.0.
- 📦 Bump RR2 to v2.0.1.
- 📦 Bump golang version in the CI and in the `go.mod` to 1.16

v1.0.0 (02.03.2021)
-------------------

- ⬆️ Update temporal in the `docker-compose.yaml` to `1.7.0`.
- ⬆️ Endure update to `v1.0.0-RC.2`
- ⬆️ RR update to `v2.0.0`

v1.0.0-RC.2 (17.02.2021)
-------------------

- Update `docker-compose.yaml`, use `postgres` instead of `cassandra`.
- Endure update to v1.0.0-RC.2
- Roadrunner core update to v2.0.0-RC.3 (ref: [release](https://github.com/spiral/roadrunner/releases/tag/v2.0.0-RC.3))

v1.0.0-RC.1 (11.02.2021)
-------------------

- RR-Core to v2.0.0-RC.1
- Endure update to v1.0.0-beta.23
- Change `RR_MODE` to `temporal` in the `activity` and `workflow` plugins.
- Non significant improvements (comments, errors.E usage, small logical issues)
