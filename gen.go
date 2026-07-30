package dji_rc_joystick

//go:generate bash -c "rm -rf target/*"

//go:generate go run ./_generate/language mod/data/language/translations target/language/languages_gen.go language

//go:generate bash -c "go run github.com/amazing-generators/gometagen/cmd/gometagen@$DOLLAR{GOMETAGEN_VERSION:-latest} generate -source . -format go -out target -force"
//go:generate bash -c "go run github.com/amazing-generators/godepsgen/cmd/godepsgen@$DOLLAR{GODEPSGEN_VERSION:-latest} -source . -pkg target -out target -skip-licenses -force"
//go:generate bash -c "go run github.com/amazing-generators/godepsgen/cmd/godepsgen@$DOLLAR{GODEPSGEN_VERSION:-latest} -source cli/go.mod -pkg cli -out target/cli/dependencies_gen.go -skip-licenses -force"
//go:generate bash -c "go run github.com/amazing-generators/godepsgen/cmd/godepsgen@$DOLLAR{GODEPSGEN_VERSION:-latest} -source gui/go.mod -pkg gui -out target/gui/dependencies_gen.go -force"
//go:generate bash -c "go run github.com/amazing-generators/goconfgen/cmd/goconfgen@$DOLLAR{GOCONFGEN_VERSION:-latest} -schema settings.yml -out target/settings -pkg settings -formats json,yaml -with-presets=false -with-interfaces=false -force"
