package sysinfo

import "testing"

func TestSystemdExecArgvAndJackParsing(t *testing.T) {
	raw := `{ path=/usr/bin/jackd ; argv[]=/usr/bin/jackd -R -P95 -dalsa -dhw:CARD=Audio,DEV=0 -r48000 -p256 -n2 -X seq ; ignore_errors=no ; }`
	args := systemdExecArgv(raw)
	cfg, err := parseJackdArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Device != "hw:CARD=Audio,DEV=0" || cfg.SR != 48000 || cfg.Buf != 256 || cfg.Periods != 2 || cfg.RTPrio != 95 || !cfg.RT {
		t.Fatalf("unexpected JACK config: %#v", cfg)
	}
}

func TestSystemctlIsActiveArgs(t *testing.T) {
	got := systemctlIsActiveArgs("jackd")
	want := []string{"is-active", "jackd"}

	if len(got) != len(want) {
		t.Fatalf("unexpected argument count: got %q want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected systemctl arguments: got %q want %q", got, want)
		}
	}
}
