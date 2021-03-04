CHANGELOG
=========

vnext
-------------------
-🪛 Use `runCommand` instead of `pushCommand` due to phantom unexpected responses in the workflow process `Close` method.

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
