# Contributing

Thanks for improving DJI RC Joystick.

## Start from the development tree

Start contribution work from `main` in the canonical repository. Fork
`voluminor/dji-rc-joystick`, then clone the fork and create a focused branch:

```bash
git clone https://github.com/<your-user>/dji-rc-joystick.git
cd dji-rc-joystick
git remote add upstream https://github.com/voluminor/dji-rc-joystick.git
git fetch upstream
git checkout -B main upstream/main
git checkout -b feat/short-description
```

Do not base development work on a release tag or its GitHub source archive. A
release tag embeds generated `target/` packages and `go.sum` so the published
library can be built, then removes development-only content such as application
sources, CI workflows, generators, scripts, and workspace files. It is a
distribution tree, not the complete development tree required for a pull
request.

## Requirements

- Go 1.26 or newer.
- Git, Bash, and network access for modules and generators.
- A C compiler and the platform libraries required by Fyne when developing the
  GUI.

## Bootstrap

On a new checkout, run:

```bash
bash _run/firststart.sh
```

The script installs the local workflow, synchronizes the workspace, and
generates `target/`. Run `go generate .` after changing `go.mod`, `cli/go.mod`,
`gui/go.mod`, translations, or `settings.yml`.

Generated files are not edited manually. Update their source, regenerate them,
and review the generated result.

## Test and verify

Run the relevant checks before opening a pull request:

```bash
CGO_ENABLED=1 go test -race ./...
CGO_ENABLED=1 go -C cli test -race ./...
CGO_ENABLED=1 go -C gui test -race ./...

go vet ./...
golangci-lint run ./...

(
cd cli
go vet ./...
golangci-lint run ./...
)

(
cd gui
go vet ./...
golangci-lint run ./...
)
```

The GUI needs a working CGO and Fyne development environment. If that is not
available locally, state the limitation in the pull request and run the
remaining checks.

## Pull requests

Keep a pull request focused. Explain the user-visible change, update tests and
documentation when behavior changes, and include the exact commands you ran.
Do not include unrelated generated files or formatting-only changes.

Report suspected security issues privately through [SECURITY.md](SECURITY.md),
not in an issue or pull request.
