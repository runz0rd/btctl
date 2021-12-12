package btctl

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

const (
	isPoweredIndicator   = "Powered: yes"
	isConnectedIndicator = "Connected: yes"
)

type BluetoothCtl exec.Cmd

func NewBluetoothCtl() (*BluetoothCtl, error) {
	// check for bluetoothctl
	_, err := exec.LookPath("bluetoothctl")
	if err != nil {
		return nil, err
	}
	return &BluetoothCtl{}, nil
}

func (_ BluetoothCtl) PowerOn(ctx context.Context) error {
	if out, err := exec.CommandContext(ctx, "bluetoothctl", "power", "on").CombinedOutput(); err != nil {
		return errors.WithMessage(err, string(out))
	}
	return nil
}

func (_ BluetoothCtl) PowerOff(ctx context.Context) error {
	if out, err := exec.CommandContext(ctx, "bluetoothctl", "power", "off").CombinedOutput(); err != nil {
		return errors.WithMessage(err, string(out))
	}
	return nil
}

func (_ BluetoothCtl) Disconnect(ctx context.Context) error {
	if out, err := exec.CommandContext(ctx, "bluetoothctl", "disconnect").CombinedOutput(); err != nil {
		return errors.WithMessage(err, string(out))
	}
	return nil
}

func (_ BluetoothCtl) Connect(ctx context.Context, device string) error {
	if out, err := exec.CommandContext(ctx, "bluetoothctl", "connect", device).CombinedOutput(); err != nil {
		return errors.WithMessage(err, string(out))
	}
	return nil
}

func (_ BluetoothCtl) IsConnected(ctx context.Context, device string) (bool, error) {
	out, err := exec.CommandContext(ctx, "bluetoothctl", "info", device).CombinedOutput()
	if err != nil {
		return false, errors.WithMessage(err, string(out))
	}
	if strings.Contains(string(out), isConnectedIndicator) {
		return true, nil
	}
	return false, nil
}

func (_ BluetoothCtl) IsPowered(ctx context.Context) (bool, error) {
	out, err := exec.CommandContext(ctx, "bluetoothctl", "show").CombinedOutput()
	if err != nil {
		return false, errors.WithMessage(err, string(out))
	}
	if strings.Contains(string(out), isPoweredIndicator) {
		return true, nil
	}
	return false, nil
}

func (_ BluetoothCtl) Devices(ctx context.Context) (map[string]string, error) {
	out, err := exec.CommandContext(ctx, "bluetoothctl", "devices").CombinedOutput()
	if err != nil {
		return nil, errors.WithMessage(err, string(out))
	}
	devices := make(map[string]string)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		pieces := strings.Split(line, " ")
		if len(pieces) == 1 {
			return nil, fmt.Errorf("line does not contain %q", " ")
		}
		name := strings.Join(pieces[2:], " ")
		device := pieces[1]
		devices[name] = device
	}
	return devices, nil
}
