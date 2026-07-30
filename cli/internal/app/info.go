package app

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
	"github.com/voluminor/dji-rc-joystick/target"
	clidependencies "github.com/voluminor/dji-rc-joystick/target/cli"
)

// // // // // // // // // //

type profileResultObj struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type infoResultObj struct {
	Schema       int                `json:"schema"`
	Name         string             `json:"name"`
	Version      string             `json:"version"`
	Hash         string             `json:"hash"`
	Profiles     []profileResultObj `json:"profiles"`
	Dependencies map[string]string  `json:"dependencies"`
}

func buildInfoResult() infoResultObj {
	profiles := rc.Profiles()
	profileResults := make([]profileResultObj, 0, len(profiles))
	for _, profile := range profiles {
		profileResults = append(profileResults, profileResultObj{
			ID: profile.ID, Name: profile.Name,
		})
	}

	versions := make(map[string]string, len(clidependencies.VersionByModule))
	for name, version := range clidependencies.VersionByModule {
		versions[name] = version
	}
	return infoResultObj{
		Schema: output.SchemaVersion, Name: target.Name,
		Version: target.Version, Hash: target.Hash,
		Profiles: profileResults, Dependencies: versions,
	}
}

func writeInfoText(dst io.Writer, style output.StyleObj, info infoResultObj) error {
	var builder strings.Builder
	builder.WriteString(title(style))
	builder.WriteString("\n\n")
	builder.WriteString(style.Bold(style.Magenta("Supported remotes")))
	builder.WriteString("\n")
	for _, profile := range info.Profiles {
		builder.WriteString("  ")
		builder.WriteString(style.Cyan(fmt.Sprintf("%-8s", profile.ID)))
		builder.WriteString(profile.Name)
		builder.WriteString("\n")
	}

	names := make([]string, 0, len(info.Dependencies))
	width := 0
	for name := range info.Dependencies {
		names = append(names, name)
		if len(name) > width {
			width = len(name)
		}
	}
	sort.Strings(names)
	builder.WriteString("\n")
	builder.WriteString(style.Bold(style.Magenta("Dependencies")))
	builder.WriteString(style.Dim(fmt.Sprintf(" (%d)", len(names))))
	builder.WriteString("\n")
	for _, name := range names {
		builder.WriteString("  ")
		builder.WriteString(style.Cyan(fmt.Sprintf("%-*s", width+2, name)))
		builder.WriteString(style.Blue(info.Dependencies[name]))
		builder.WriteString("\n")
	}

	builder.WriteString("\nRun \"")
	builder.WriteString(style.Cyan("--help"))
	builder.WriteString("\" for detailed usage.\n")
	if _, err := io.WriteString(dst, builder.String()); err != nil {
		return fmt.Errorf("write project information: %w", err)
	}
	return nil
}

func executeInfo(options infoOptionsObj, dependencies DependenciesObj) resultObj {
	info := buildInfoResult()
	if options.format == formatJSON {
		if err := output.WriteJSON(dependencies.Stdout, info); err != nil {
			return resultObj{code: exitRuntime, err: fmt.Errorf("write project information: %w", err)}
		}
		return resultObj{code: exitSuccess}
	}

	style, err := styleFor(
		options.color.value,
		dependencies.StdoutTTY,
		dependencies.NoColor && !options.color.set,
	)
	if err != nil {
		return resultObj{code: exitUsage, err: err}
	}
	if err = writeInfoText(dependencies.Stdout, style, info); err != nil {
		return resultObj{code: exitRuntime, err: err}
	}
	return resultObj{code: exitSuccess}
}
