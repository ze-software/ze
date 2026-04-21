package l2tpshaper

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

// VALIDATES: AC-8 -- TBF qdisc type parsed from config.
func TestShaperConfigParsing_TBF(t *testing.T) {
	data := `{"shaper":{"qdisc-type":"tbf","default-rate":"10mbit"}}`
	cfg, found, err := parseShaperConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected shaper config to be found")
	}
	if cfg.QdiscType != traffic.QdiscTBF {
		t.Errorf("qdisc type: got %v, want TBF", cfg.QdiscType)
	}
	if cfg.DefaultRate != 10_000_000 {
		t.Errorf("rate: got %d, want 10000000", cfg.DefaultRate)
	}
}

// VALIDATES: AC-9 -- HTB qdisc type parsed from config.
func TestShaperConfigParsing_HTB(t *testing.T) {
	data := `{"shaper":{"qdisc-type":"htb","default-rate":"100mbit","upload-rate":"50mbit"}}`
	cfg, found, err := parseShaperConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected shaper config to be found")
	}
	if cfg.QdiscType != traffic.QdiscHTB {
		t.Errorf("qdisc type: got %v, want HTB", cfg.QdiscType)
	}
	if cfg.DefaultRate != 100_000_000 {
		t.Errorf("download rate: got %d, want 100000000", cfg.DefaultRate)
	}
	if cfg.UploadRate != 50_000_000 {
		t.Errorf("upload rate: got %d, want 50000000", cfg.UploadRate)
	}
}

// VALIDATES: AC-10 -- no shaper block means not found.
func TestShaperConfigParsing_NoBlock(t *testing.T) {
	data := `{"pool":{"ipv4":{"start":"10.0.0.1"}}}`
	_, found, err := parseShaperConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected shaper config to NOT be found")
	}
}

func TestShaperConfigParsing_EmptyData(t *testing.T) {
	_, found, err := parseShaperConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected not found for empty data")
	}
}

func TestShaperConfigValidation_ZeroRate(t *testing.T) {
	data := `{"shaper":{"qdisc-type":"tbf","default-rate":"0bit"}}`
	_, _, err := parseShaperConfig(data)
	if err == nil {
		t.Fatal("expected error for zero rate")
	}
}

func TestShaperConfigValidation_BadQdisc(t *testing.T) {
	data := `{"shaper":{"qdisc-type":"fq_codel","default-rate":"10mbit"}}`
	_, _, err := parseShaperConfig(data)
	if err == nil {
		t.Fatal("expected error for unsupported qdisc type")
	}
}

func TestShaperConfigValidation_MissingRate(t *testing.T) {
	data := `{"shaper":{"qdisc-type":"tbf"}}`
	_, _, err := parseShaperConfig(data)
	if err == nil {
		t.Fatal("expected error for missing default-rate")
	}
}

// VALIDATES: default qdisc is tbf when omitted.
func TestShaperConfigParsing_DefaultQdisc(t *testing.T) {
	data := `{"shaper":{"default-rate":"5mbit"}}`
	cfg, found, err := parseShaperConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected found")
	}
	if cfg.QdiscType != traffic.QdiscTBF {
		t.Errorf("default qdisc: got %v, want TBF", cfg.QdiscType)
	}
}
