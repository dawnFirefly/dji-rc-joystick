package windows

import (
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

func validateIPAddress(value string) error {
	if net.ParseIP(strings.TrimSpace(value)) == nil {
		return errors.New("enter a valid IP address")
	}
	return nil
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port < 1 || port > 65535 {
		return 0, errors.New("enter a port from 1 to 65535")
	}
	return port, nil
}

func formatRemoteOption(remote rc.RemoteObj) string {
	parts := []string{remote.IP}
	if remote.Model != "" {
		parts = append(parts, remote.Model)
	}
	if remote.Firmware != "" {
		parts = append(parts, remote.Firmware)
	}
	return strings.Join(parts, " — ")
}
