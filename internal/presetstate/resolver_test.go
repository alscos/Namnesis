package presetstate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMissingNAMModelFromPreset(t *testing.T) {
	dir := t.TempDir()
	preset := "12_Fender_deluxe_rvb"
	body := "SetPreset 12_Fender_deluxe_rvb\n" +
		"SetParam NAM Model \"Fender_Deluxe_reverb-5V-ESR-009-S-300-EPOCS\"\n" +
		"SetParam Cabinet Impulse \"Friedman_2x12_D120_Mix18_yrk_Audio\"\n"
	if err := os.WriteFile(filepath.Join(dir, preset), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	r := New([]string{dir})
	resolved := r.Resolve("SetPreset 12_Fender_deluxe_rvb\r\nSetParam NAM Model \r\nSetParam Cabinet Impulse \"LiveCab\"\r\n")

	if got := resolved["NAM"]["Model"]; got != "Fender_Deluxe_reverb-5V-ESR-009-S-300-EPOCS" {
		t.Fatalf("resolved NAM model = %q", got)
	}
	if _, ok := resolved["Cabinet"]; ok {
		t.Fatalf("Cabinet should not be resolved when Dump Program already has a value: %#v", resolved)
	}
}

func TestUnsafePresetNameIsRejected(t *testing.T) {
	r := New([]string{t.TempDir()})
	if got := r.Resolve("SetPreset ../escape\nSetParam NAM Model\n"); got != nil {
		t.Fatalf("unsafe preset unexpectedly resolved: %#v", got)
	}
}
