module github.com/voluminor/dji-rc-joystick/cli

go 1.26

require (
	github.com/fatih/color v1.19.0
	github.com/mattn/go-isatty v0.0.24
	github.com/voluminor/dji-rc-joystick v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/mattn/go-colorable v0.1.15 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace github.com/voluminor/dji-rc-joystick => ../
