# Template Resilience

Jinja2 templates in HA can return `unavailable` or `unknown` and raise errors on None values.
These patterns prevent automation failures and silent incorrect behavior.

## Always guard numeric sensors with `has_value()`

```yaml
# FRAGILE — fails when sensor is unavailable
"{{ states('sensor.temperature') | float > 22 }}"

# RESILIENT — short-circuits when sensor has no valid value
"{{ states.sensor.temperature | has_value and states('sensor.temperature') | float > 22 }}"
```

`has_value()` returns `false` if the state is `unavailable`, `unknown`, `None`, or empty.

## Check for unavailable before math or string ops

```yaml
# Pattern: guard then compute
{% set temp = states('sensor.temperature') %}
{% if temp not in ['unavailable', 'unknown', 'none', ''] %}
  {{ temp | float | round(1) }}
{% else %}
  N/A
{% endif %}
```

Or using the state object:
```yaml
{% if states.sensor.temperature is defined and states.sensor.temperature.state not in ['unavailable', 'unknown'] %}
  {{ states.sensor.temperature.state | float }}
{% endif %}
```

## Elimination logic pattern

When you need a fallback chain across multiple sensors:

```yaml
{% set sources = [
    states('sensor.indoor_temp_main'),
    states('sensor.indoor_temp_backup')
  ] | select('ne', 'unavailable') | select('ne', 'unknown') | list %}
{% if sources %}
  {{ sources[0] | float }}
{% else %}
  unavailable
{% endif %}
```

## Validate before deploying

Use `render_template` to test your template against the live HA instance before embedding it in an automation:

```json
{
  "tool": "render_template",
  "template": "{{ states.sensor.temperature | has_value and states('sensor.temperature') | float > 22 }}"
}
```

Expected: `true` or `false` (not an error string like `TemplateError`).
If `render_template` returns an error, fix the template before using it in an automation or helper.

## Template sensors and binary sensors

When Config Entry helpers (`statistics`, `trend`, `utility_meter`, `filter`, `generic_thermostat`) need a sensor source but you only have an `input_number`, wrap it:

```yaml
# Template sensor wrapping input_number.my_value
template:
  - sensor:
      - name: my_value_sensor
        state: "{{ states('input_number.my_value') }}"
        unit_of_measurement: "W"
        device_class: power
```

Then use `sensor.my_value_sensor` as the source entity for Config Entry helpers.

## Common pitfalls

| Symptom                                      | Cause                                        | Fix                                               |
| -------------------------------------------- | -------------------------------------------- | ------------------------------------------------- |
| Template returns `None` or `0` unexpectedly  | Sensor is `unavailable`; `\| float` returns 0 | Add `has_value()` guard before the cast           |
| Automation runs but condition always passes  | Forgot `\| float` after `states()`            | Cast to float/int before comparing to a number    |
| `render_template` returns TemplateError      | Undefined variable or wrong attribute name   | Check entity ID spelling; verify attribute exists |
| Template sensor always `unavailable`         | Source sensor domain not accepted            | Wrap with a template sensor if source is input_*  |
