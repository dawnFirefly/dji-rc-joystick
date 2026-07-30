//go:build cgo || ci

package app

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/voluminor/dji-rc-joystick/gui/internal/logging"
	"github.com/voluminor/dji-rc-joystick/gui/internal/theme"
	"github.com/voluminor/dji-rc-joystick/gui/internal/windows"
	"github.com/voluminor/dji-rc-joystick/mod/data"
	"github.com/voluminor/dji-rc-joystick/target"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
)

// // // // // // // // // //

//go:embed icon.png
var iconData []byte

var icon = fyne.NewStaticResource("icon.png", iconData)

// //

// Run assembles and starts the desktop application.
func Run(args []string) (resultErr error) {
	dataObj, err := data.New(args)
	if err != nil {
		return err
	}

	// //

	loggingObj, err := logging.New(dataObj.Logging(), dataObj.ConfigPath())
	if err != nil {
		return fmt.Errorf("initialize logging: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, loggingObj.Close())
	}()
	if err = loggingObj.RedirectStandard("fyne"); err != nil {
		return fmt.Errorf("redirect standard logging: %w", err)
	}

	rootLogger := loggingObj.Logger()
	logger := rootLogger.With().
		Str("component", "app").
		Logger()
	loggingConfig := dataObj.Logging()

	logger.Info().
		Str("version", target.Version).
		Str("version_major", target.VersionMajor).
		Uint16("version_minor", target.VersionMinor).
		Uint16("version_patch", target.VersionPatch).
		Str("date_update", target.DateUpdate).
		Str("hash", target.Hash).
		Str("go_version", runtime.Version()).
		Str("go_os", runtime.GOOS).
		Str("go_arch", runtime.GOARCH).
		Int("pid", os.Getpid()).
		Str("config_path", dataObj.ConfigPath()).
		Str("log_path", loggingObj.FilePath()).
		Bool("console_enabled", loggingConfig.Console.Enabled).
		Str("console_level", loggingConfig.Console.Level.String()).
		Bool("file_enabled", loggingConfig.File.Enabled).
		Str("file_level", loggingConfig.File.Level.String()).
		Str("file_dir", loggingConfig.File.Dir).
		Str("file_max_size", loggingConfig.File.MaxSize.String()).
		Uint("file_max_backups", loggingConfig.File.MaxBackups).
		Str("file_max_age", loggingConfig.File.MaxAge.String()).
		Bool("file_compress", loggingConfig.File.Compress).
		Msg("Application started")

	// //

	application := fyneapp.NewWithID(target.Name)
	application.SetIcon(icon)

	themeObj := theme.New(dataObj.UI())
	application.Settings().SetTheme(themeObj)

	// //

	window := windows.New(application, rootLogger, dataObj, themeObj, icon)
	window.Window().SetMaster()

	window.Window().ShowAndRun()
	logger.Info().Msg("Application stopped")
	return nil
}
