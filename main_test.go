package main

import (
	"testing"
)

func Test_run(t *testing.T) {
	type args struct {
		connected    string
		disconnected string
		off          string
		device       string
		storePath    string
		command      string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"menu trigger", args{"c", "dc", "off", "", "/tmp/.btdev", "toggle"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args.connected, tt.args.disconnected, tt.args.off, tt.args.device, tt.args.storePath, tt.args.command); (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
