package selfmanage_test

import (
	"testing"

	"github.com/djcp/enplace/internal/selfmanage"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		path string
		want selfmanage.Method
	}{
		{"homebrew arm cellar", "/opt/homebrew/Cellar/enplace/1.4.0/bin/enplace", selfmanage.MethodHomebrew},
		{"homebrew arm shim", "/opt/homebrew/bin/enplace", selfmanage.MethodHomebrew},
		{"homebrew intel cellar", "/usr/local/Cellar/enplace/1.4.0/bin/enplace", selfmanage.MethodHomebrew},
		{"linuxbrew", "/home/linuxbrew/.linuxbrew/bin/enplace", selfmanage.MethodHomebrew},
		{"scoop app", `C:\Users\dan\scoop\apps\enplace\current\enplace.exe`, selfmanage.MethodScoop},
		{"scoop shim", `C:\Users\dan\scoop\shims\enplace.exe`, selfmanage.MethodScoop},
		{"manual local bin", "/usr/local/bin/enplace", selfmanage.MethodUnknown},
		{"manual home bin", "/home/dan/.local/bin/enplace", selfmanage.MethodUnknown},
		{"windows programs", `C:\Users\dan\AppData\Local\Programs\enplace\enplace.exe`, selfmanage.MethodUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selfmanage.Detect(tc.path); got != tc.want {
				t.Errorf("Detect(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestMethodManagesBinary(t *testing.T) {
	if selfmanage.MethodUnknown.ManagesBinary() {
		t.Error("MethodUnknown should not manage the binary")
	}
	if !selfmanage.MethodHomebrew.ManagesBinary() {
		t.Error("MethodHomebrew should manage the binary")
	}
	if !selfmanage.MethodScoop.ManagesBinary() {
		t.Error("MethodScoop should manage the binary")
	}
}

func TestUpgradeCommand(t *testing.T) {
	if got := selfmanage.MethodHomebrew.UpgradeCommand(); got != "brew upgrade enplace" {
		t.Errorf("homebrew upgrade cmd = %q", got)
	}
	if got := selfmanage.MethodScoop.UpgradeCommand(); got != "scoop update enplace" {
		t.Errorf("scoop upgrade cmd = %q", got)
	}
	if got := selfmanage.MethodUnknown.UpgradeCommand(); got != "" {
		t.Errorf("manual upgrade cmd = %q, want empty", got)
	}
}
