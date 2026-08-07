---
kind: note
level:
stage:
---
The plugin config framework delivers every YANG leaf value to a plugin's `ParseConfig` as a JSON **string** (`"true"`, `"50000"`, `"3.5"`), never the native JSON type. A hand-written parser that coerces a config value with a native-type assertion always fails the assertion on that string and silently falls back to the leaf's **default**:
