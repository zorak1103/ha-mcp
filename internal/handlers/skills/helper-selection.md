# Helper Selection

26 helper types are available via `manage_helper action=create`. Choosing the right type and knowing how entity IDs are derived prevents the most common helper mistakes.

## Entity ID rules (critical)

The parameter that controls the entity ID differs by helper group:

| Helper types                                                                                                                                                               | Entity ID controlled by         | Example                                               |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------- | ----------------------------------------------------- |
| `input_boolean`, `input_number`, `input_text`, `input_select`, `input_datetime`, `input_button`, `counter`, `timer`, `schedule`                                            | `id` parameter                  | `id: my_switch` → `input_boolean.my_switch`           |
| `threshold`, `derivative`, `integral`, `group`, `template_sensor`, `template_binary_sensor`, `utility_meter`, `min_max`, `statistics`, `trend`, `random_sensor`, `random_binary_sensor`, `filter`, `tod`, `generic_thermostat`, `generic_hygrostat`, `switch_as_x` | `name` parameter (HA slugifies) | `name: "My Sensor"` → `sensor.my_sensor` (ASCII only) |

Non-ASCII characters in `name` are stripped during slugification. Use ASCII names for predictable entity IDs with Config Entry helpers.

## Decision matrix

| Need                                           | Helper type              | Notes                                          |
| ---------------------------------------------- | ------------------------ | ---------------------------------------------- |
| On/off toggle the user controls                | `input_boolean`          | Use `id` param for entity_id                   |
| Numeric slider / setpoint                      | `input_number`           | Requires `min` and `max` on both create/update |
| Free-text field                                | `input_text`             | Optional `min`/`max` length, `pattern`         |
| Dropdown list                                  | `input_select`           | Requires `options` array                       |
| Date or time picker                            | `input_datetime`         | `has_date`, `has_time` booleans                |
| Pressable button                               | `input_button`           | No state; triggers `pressed` event             |
| Count events                                   | `counter`                | Optional `step`, `min`, `max`                  |
| Countdown timer                                | `timer`                  | Optional `duration` in `HH:MM:SS`             |
| Time-of-day schedule                           | `schedule`               | Calendar-style blocks                          |
| Computed sensor based on another sensor        | `template_sensor`        | `state_template`, optional `unit`              |
| Binary sensor derived from logic               | `template_binary_sensor` | `state_template`, optional `device_class`      |
| Alert when sensor crosses threshold            | `threshold`              | Source must be `sensor.*`                      |
| Rate of change of a sensor                     | `derivative`             | Source must be `sensor.*`                      |
| Energy / power integration (kWh)               | `integral`               | Source must be `sensor.*`                      |
| Group of entities into one state               | `group`                  | `entities` array                               |
| Track daily/weekly energy usage                | `utility_meter`          | Source must be `sensor.*`; `cycle` required    |
| Min/max/mean of multiple sensors               | `min_max`                | `entity_ids` array, `min_max_type` required    |
| Statistical aggregation over time              | `statistics`             | Source must be `sensor.*`; 3-step flow         |
| Rising/falling trend detection                 | `trend`                  | Source must be `sensor.*`                      |
| Random number sensor                           | `random_sensor`          | `minimum`, `maximum`                           |
| Random binary sensor                           | `random_binary_sensor`   | `chance` (0–100)                               |
| Filter noisy sensor readings                   | `filter`                 | Source must be `sensor.*`; 2-step flow         |
| Time-of-day binary (active between times)      | `tod`                    | `after`, `before` times                        |
| Thermostat from switch + sensor                | `generic_thermostat`     | Switch must be `switch.*`, sensor `sensor.*`   |
| Humidistat from switch + sensor                | `generic_hygrostat`      | Switch must be `switch.*`, sensor `sensor.*`   |
| Expose a switch as a cover/fan/light/etc.      | `switch_as_x`            | Switch must be `switch.*`                      |

## Source entity domain requirements

Config Entry helpers that need a sensor (`threshold`, `derivative`, `integral`, `statistics`, `trend`, `filter`, `utility_meter`, `min_max`) require a `sensor.*` entity — not `input_number.*`.

Workaround: wrap `input_number.my_value` in a `template_sensor` that reads its state, then use the template sensor as the source.

Config Entry helpers that need a switch (`generic_thermostat`, `generic_hygrostat`, `switch_as_x`) require a `switch.*` entity — not `input_boolean.*`.

Workaround: wrap `input_boolean.my_flag` in a `template_binary_sensor` with `turn_on`/`turn_off` service actions.

## WebSocket helper updates require ALL mandatory fields

When updating WebSocket-based helpers (`input_*`, `counter`, `timer`, `schedule`), include all mandatory fields in every update — not just the ones changing. For example, `input_number` always needs `min` and `max` even if only `name` is changing.

Use `manage_helper action=get_details` to fetch the current config, then merge your changes before sending the update.
