package windows

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/voluminor/dji-rc-joystick/target"
	guidependencies "github.com/voluminor/dji-rc-joystick/target/gui"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// // // // // // // // // //

const fallbackAboutDescription = "The application finds DJI remote controllers on the local network or connects to an IP address. " +
	"It displays controller details and the state of sticks, dials, the flight-mode switch, and buttons. " +
	"Virtual-gamepad axes and buttons are mapped separately, analog controls can be inverted, and the mapping is saved in the configuration.\n\n" +
	"Discovery, connection, and monitoring work on Linux, Windows, and macOS. " +
	"Linux emulation uses /dev/uinput and requests system authorization when direct access is unavailable. " +
	"A configured vJoy device is required on 64-bit Windows. " +
	"macOS and other platforms do not support a virtual gamepad.\n\n" +
	"DJI RC and DJI RC 2 have been verified. " +
	"DJI RC Pro, DJI Smart Controller, and DJI RC Pro Enterprise are expected to be compatible, but remain untested. " +
	"Discovery, the complete set of controls, and connection stability are not guaranteed for those models."

// //

var sourceURL = &url.URL{
	Scheme: "https",
	Host:   "github.com",
	Path:   "/voluminor/dji-rc-joystick",
}

func (obj *Obj) moduleNames() []string {
	result := make([]string, 0, len(guidependencies.VersionByModule))
	for name := range guidependencies.VersionByModule {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func (obj *Obj) moduleAccordion() *widget.Accordion {
	accordion := widget.NewAccordion()
	for _, name := range obj.moduleNames() {
		licenseText := obj.lg("about.license.empty", "License text is unavailable.")
		if license := guidependencies.LicenseByModule[name]; license != nil {
			if value := license.String(); value != "" {
				licenseText = value
			}
		}

		detail := widget.NewLabel(licenseText)
		detail.Wrapping = fyne.TextWrapWord
		title := fmt.Sprintf("%s %s", name, guidependencies.VersionByModule[name])
		accordion.Append(widget.NewAccordionItem(title, detail))
	}

	return accordion
}

func (obj *Obj) renderAbout() fyne.CanvasObject {
	title := widget.NewLabel(target.Name)
	title.SizeName = theme.SizeNameHeadingText
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Importance = widget.SuccessImportance

	version := widget.NewLabel(target.Version)
	version.Importance = widget.LowImportance
	date := widget.NewLabel(target.DateUpdate)
	date.Importance = widget.LowImportance
	source := widget.NewHyperlink("GitHub", sourceURL)

	description := widget.NewLabel(obj.lg(
		"about.description",
		fallbackAboutDescription,
	))
	description.Wrapping = fyne.TextWrapWord

	modulesTitle := widget.NewLabel(obj.lg("about.modules", "Dependencies:"))
	modulesTitle.TextStyle = fyne.TextStyle{Bold: true}
	modulesCount := widget.NewLabel(strconv.Itoa(len(guidependencies.VersionByModule)))
	modulesCount.Importance = widget.LowImportance

	content := container.NewVBox(
		container.NewBorder(nil, nil, container.NewHBox(title, version, date), source),
		description,
		widget.NewSeparator(),
		container.NewBorder(nil, nil, modulesTitle, modulesCount),
		obj.moduleAccordion(),
	)

	return container.NewVScroll(content)
}
