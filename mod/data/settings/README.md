# Settings

Package `settings` loads, validates, snapshots, and atomically replaces the
application settings file. It also converts generated mapping settings into
the runtime mapping used by `mod/rc`.

Import path:

```go
import datasettings "github.com/voluminor/dji-rc-joystick/mod/data/settings"
```

## Load and save flow

```mermaid
flowchart TD
    subgraph Load["New(args)"]
        Defaults["generated defaults"] --> File{"settings file exists?"}
        File -->|yes| Parse["parse JSON or YAML"]
        File -->|no| Current["use defaults"]
        Parse --> Current
        Current --> CLI["apply generated CLI overrides"]
        CLI --> Valid["generated + logging validation"]
        Valid -->|file was missing| InitialSave["create settings file"]
    end

    subgraph Save["Save(snapshot)"]
        Input["independent SnapshotObj"] --> Validate["generated + logging validation"]
        Validate --> Render["render by file extension"]
        Render --> Temp["same-directory temporary file"]
        Temp --> Sync["chmod 0600, write, fsync, close"]
        Sync --> Rename["atomic rename"]
        Rename --> Active["replace in-memory config"]
    end
```

Precedence is:

1. generated defaults;
2. values from the settings file;
3. generated CLI overrides.

## Paths and formats

Without an explicit path, `New` uses:

```text
<os.UserConfigDir()>/dji-rc-joystick/settings.json
```

The path can be selected with any of:

```text
--config path/to/settings.json
--config=path/to/settings.yaml
-config path/to/settings.json
-config=path/to/settings.yaml
```

The custom config flag is removed before the remaining arguments are passed to
the generated settings parser. A second config flag, a missing value, or an
empty path is rejected.

Supported extensions are `.json`, `.yaml`, and `.yml`. A file larger than
1 MiB, a non-regular file, or an unsupported extension is rejected before
parsing. A missing file is created with the resolved defaults and CLI
overrides.

## Public API

| API                       | Contract                                                                                |
|---------------------------|-----------------------------------------------------------------------------------------|
| `type Obj`                | Owns one schema/logging-validated configuration and its resolved path.                  |
| `type SnapshotObj`        | Independent mutable value containing `Logging`, `UI`, and `Mapping`.                    |
| `New(args)`               | Loads precedence layers, runs schema/logging validation, and creates a missing file.    |
| `(*Obj).Path()`           | Returns the immutable resolved file path.                                               |
| `(*Obj).Snapshot()`       | Returns an independent value; mutations do not affect active settings.                  |
| `(*Obj).Save(snapshot)`   | Runs schema/logging validation and commits the snapshot before publishing it in memory. |
| `ControlMapping(mapping)` | Converts `target/settings.MappingObj` into a fresh `rc.MappingObj`.                     |
| `(*Obj).ControlMapping()` | Converts the active mapping snapshot.                                                   |

`SnapshotObj` intentionally contains only the public settings values. Internal
origin metadata from the generated parser is neither shared nor exposed.

`ControlMapping` allocates fresh maps. Validate the result with
`rc.MappingObj.Validate`, or pass it to `rc.New`, which validates it and takes
its own copy.

`New` and `Save` do not validate mapping action names or source/action
compatibility. They preserve those strings; runtime mapping validation is a
separate step.

## Example

```go
store, err := datasettings.New(os.Args[1:])
if err != nil {
	return err
}

snapshot := store.Snapshot()
snapshot.Logging.Console.Level = targetsettings.ConsoleLogLevelDebug
snapshot.UI.TextSize = 1.25
snapshot.UI.DarkMode = false

if err := store.Save(snapshot); err != nil {
	return err
}

mapping := store.ControlMapping()
if err := mapping.Validate(); err != nil {
	return err
}
```

## Persistence guarantees

- Directory creation requests mode `0700`; the final new directory is chmodded
  to `0700`.
- The temporary file is chmodded to `0600`, and the renamed destination keeps
  that mode on platforms with POSIX permissions.
- Existing directories are not chmodded.
- Rendering and schema/logging validation happen before the active snapshot
  changes.
- Logging validation requires at least one enabled output and a non-empty file
  directory.
- The temporary file is created beside the destination, flushed with
  `fsync`, closed, and renamed over the destination.
- If rendering, writing, syncing, closing, or renaming fails, the in-memory
  snapshot is unchanged and the temporary file is removed.

The rename provides atomic replacement on supported local filesystems. The
package does not claim full power-loss durability for the directory entry
because it does not fsync the parent directory.

## Concurrency and resource bounds

`Obj` uses an `RWMutex`. Snapshot reads may proceed concurrently; saves are
serialized and hold the write lock during the filesystem transaction. Input
files and rendered output are capped at 1 MiB. `Save` renders a caller-supplied
snapshot before checking that output size, so callers should not pass
unbounded strings.

The package follows symlink and filesystem behavior supplied by the operating
system. A caller that accepts an untrusted custom path must enforce its own
directory policy.

## Errors

Errors identify the failing stage: path resolution, open/stat/read, format
lookup, parse, CLI application, validation, render, directory creation,
temporary write, sync, close, or rename. A failed `Save` never silently updates
only the in-memory state.
