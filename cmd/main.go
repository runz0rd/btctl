package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	isPoweredIndicator   = "Powered: yes"
	isConnectedIndicator = "Connected: yes"
)

func main() {
	var connectedFlag string
	var disconnectedFlag string
	var offFlag string
	var storePathFlag string
	var commandFlag string
	var deviceFlag string
	var debugFlag bool
	flag.StringVar(&connectedFlag, "c", "connected", "text to display when connected")
	flag.StringVar(&disconnectedFlag, "d", "disconnected", "text to display when disonnected")
	flag.StringVar(&offFlag, "o", "off", "text to display when off")
	flag.StringVar(&storePathFlag, "storePath", "/tmp/.btdev", "path to store device id")
	flag.StringVar(&deviceFlag, "device", "", "deviceId")
	flag.StringVar(&commandFlag, "command", "toggle", "toggle/pick/status")
	flag.BoolVar(&debugFlag, "debug", false, "debug mode")
	flag.Parse()

	if err := run(connectedFlag, disconnectedFlag, offFlag, deviceFlag, storePathFlag, commandFlag); err != nil {
		if debugFlag {
			log.Println(err)
		}
		os.Exit(1)
	}
}

func run(connected, disconnected, off, device, storePath, command string) error {
	// check for required bins
	if _, err := exec.LookPath("bluetoothctl"); err != nil {
		return err
	}
	if _, err := exec.LookPath("yad"); err != nil {
		return err
	}

	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()
	isPowered, err := isPowered(ctx)
	if err != nil {
		return err
	}
	isConnected, err := isConnected(ctx)
	if err != nil {
		return err
	}
	// defer printStatus(isPowered, isConnected, connected, disconnected, off)

	switch command {
	case "toggle":
		if device == "" {
			device, err = getLastDevice(storePath)
			if err != nil {
				return err
			}
			if device == "" {
				return fmt.Errorf("no device set in %q", storePath)
			}
		}
		if err := toggle(ctx, isConnected, isPowered, device); err != nil {
			return err
		}
	case "pick":
		if err := pick(ctx, storePath, isConnected); err != nil {
			return err
		}
	case "status":
		defer printStatus(isPowered, isConnected, connected, disconnected, off)
	default:
		return fmt.Errorf("invalid command %q provided", command)
	}
	return nil
}

func signal(ctx context.Context, process string) error {
	if out, err := exec.CommandContext(ctx, "pkill", "-SIGUSR1", process).Output(); err != nil {
		return errors.WithMessage(err, string(out))
	}
	return nil
}

// toggles between connected and powered off
func toggle(ctx context.Context, isConnected, isPowered bool, device string) error {
	if !isConnected || !isPowered {
		if err := connect(ctx, device, isConnected); err != nil {
			return err
		}
	} else {
		if _, err := exec.CommandContext(ctx, "bluetoothctl", "power", "off").CombinedOutput(); err != nil {
			return err
		}
	}
	return nil
}

// displays a selection menu for available devices
// and connects if picked
func pick(ctx context.Context, storePath string, isConnected bool) error {
	deviceMap, err := getDeviceMap(ctx)
	if err != nil {
		return err
	}
	go scanForDevices(ctx)
	_, device, err := runDeviceSelectMenu(ctx, deviceMap)
	if err != nil {
		return err
	}
	if device == "" {
		return nil
	}
	if err := connect(ctx, device, isConnected); err != nil {
		return err
	}
	if err := setLastDevice(storePath, device); err != nil {
		return err
	}
	return nil
}

func connect(ctx context.Context, device string, isConnected bool) error {
	if out, err := exec.CommandContext(ctx, "bluetoothctl", "power", "on").Output(); err != nil {
		return errors.WithMessage(err, string(out))
	}
	if isConnected {
		// this can fail if already disconnected
		if out, err := exec.CommandContext(ctx, "bluetoothctl", "disconnect").Output(); err != nil {
			return errors.WithMessage(err, string(out))
		}
	}
	if out, err := exec.CommandContext(ctx, "bluetoothctl", "connect", device).Output(); err != nil {
		return errors.WithMessage(err, string(out))
	}
	return nil
}

func printStatus(isPowered, isConnected bool, connected, disconnected, off string) {
	if !isPowered {
		fmt.Println(off)
	} else if !isConnected {
		fmt.Println(disconnected)
	} else {
		fmt.Println(connected)
	}
}

func isPowered(ctx context.Context) (bool, error) {
	output, err := exec.CommandContext(ctx, "bluetoothctl", "show").CombinedOutput()
	if err != nil {
		return false, err
	}
	if strings.Contains(string(output), isPoweredIndicator) {
		return true, nil
	}
	return false, nil
}

func isConnected(ctx context.Context) (bool, error) {
	output, _ := exec.CommandContext(ctx, "bluetoothctl", "info").CombinedOutput()
	if strings.Contains(string(output), isConnectedIndicator) {
		return true, nil
	}
	return false, nil
}

func getDeviceMap(ctx context.Context) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "bluetoothctl", "devices")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	done := make(chan bool)
	var lines []string
	go readLinesFromStdout(stdout, done, &lines)
	if cmd.Start(); err != nil {
		return nil, err
	}
	<-done
	if cmd.Wait(); err != nil {
		return nil, err
	}
	devices := make(map[string]string)
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

func getLastDevice(filepath string) (string, error) {
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", err
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(content), "\n"), nil
}

func setLastDevice(filepath, device string) error {
	if len(device) != 17 {
		return fmt.Errorf("wrong device format %q", device)
	}
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	if err != nil {
		return err
	}
	if _, err := file.WriteString(device); err != nil {
		return err
	}
	return nil
}

func runDeviceSelectMenu(ctx context.Context, deviceMap map[string]string) (name string, device string, err error) {
	cmd := exec.Command("yad", "--list", "--column=name", "--column=device", "--no-buttons", "--listen")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return name, device, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return name, device, err
	}

	done := make(chan bool)
	var lines []string
	go readLinesFromStdout(stdout, done, &lines)
	if cmd.Start(); err != nil {
		return name, device, err
	}
	go writeDevicesToStdin(ctx, stdin)
	<-done
	if cmd.Wait(); err != nil {
		return name, device, err
	}
	if len(lines) == 0 {
		// nothing selected
		return "", "", nil
	}
	pieces := strings.Split(strings.TrimRight(lines[0], "\n"), "|")
	if len(pieces) == 1 {
		return "", "", fmt.Errorf("string %q doesnt contain delimiter %q", lines[0], "|")
	}
	return pieces[0], pieces[1], nil
}

func writeDevicesToStdin(ctx context.Context, stdin io.Writer) {
	go exec.CommandContext(ctx, "bluetoothctl", "scan", "on").Output()
	added := make(map[string]string)
	for {
		deviceMap, _ := getDeviceMap(ctx)
		for name, device := range deviceMap {
			if _, ok := added[name]; ok {
				continue
			}
			io.WriteString(stdin, fmt.Sprintf("%v\n%v\n", name, device))
			added[name] = device
		}
		time.Sleep(5 * time.Second)
	}
}

func readLinesFromStdout(stdout io.ReadCloser, done chan bool, lines *[]string) {
	scanner := bufio.NewScanner(stdout)
	// Read line by line and process it
	var ls []string
	for scanner.Scan() {
		ls = append(ls, scanner.Text())
	}
	*lines = ls
	done <- true
}

func mapDiff(m, comparison map[string]string) map[string]string {
	diff := make(map[string]string)
	for k, v := range m {
		if _, ok := comparison[k]; !ok {
			diff[k] = v
		}
	}
	return diff
}

func scanForDevices(ctx context.Context) error {
	if out, err := exec.CommandContext(ctx, "bluetoothctl", "scan", "on").Output(); err != nil {
		return errors.WithMessage(err, string(out))
	}
	return nil
}
