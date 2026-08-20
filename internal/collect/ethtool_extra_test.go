//go:build linux

package collect

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"
)

func TestExtractFeatureNames(t *testing.T) {
	buf := make([]byte, 12+3*ethStringLen)
	copy(buf[12:12+ethStringLen], "tx-scatter-gather\x00")
	copy(buf[12+ethStringLen:12+2*ethStringLen], "tx-checksum-ipv4\x00")
	copy(buf[12+2*ethStringLen:12+3*ethStringLen], "rx-gro\x00")
	got := extractFeatureNames(buf, 3)
	want := []string{"tx-scatter-gather", "tx-checksum-ipv4", "rx-gro"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractFeatureNames() = %#v, want %#v", got, want)
	}
}

func TestExtractFeatureNamesTruncatedBuffer(t *testing.T) {
	buf := make([]byte, 12+ethStringLen+4)
	copy(buf[12:12+ethStringLen], "only-one\x00")
	got := extractFeatureNames(buf, 5)
	if len(got) != 1 || got[0] != "only-one" {
		t.Fatalf("extractFeatureNames() = %#v, want exactly one name", got)
	}
}

func TestParseFeatureBlocks(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e"}
	blocks := make([]byte, 16)
	binary.LittleEndian.PutUint32(blocks[0:4], 0x1f)   // available: a-e
	binary.LittleEndian.PutUint32(blocks[4:8], 0x02)   // requested: b
	binary.LittleEndian.PutUint32(blocks[8:12], 0x05)  // active: a, c
	binary.LittleEndian.PutUint32(blocks[12:16], 0x08) // never changed: d
	features := parseFeatureBlocks(blocks, names)
	if !reflect.DeepEqual(features.Hardware, []string{"a", "b", "c", "d", "e"}) {
		t.Fatalf("Hardware = %#v", features.Hardware)
	}
	if !reflect.DeepEqual(features.Wanted, []string{"b"}) {
		t.Fatalf("Wanted = %#v", features.Wanted)
	}
	if !reflect.DeepEqual(features.Active, []string{"a", "c"}) {
		t.Fatalf("Active = %#v", features.Active)
	}
	if !reflect.DeepEqual(features.NoChange, []string{"d"}) {
		t.Fatalf("NoChange = %#v", features.NoChange)
	}
}

func TestParseFeatureBlocksSecondBlockIndexing(t *testing.T) {
	names := make([]string, 40)
	for i := range names {
		names[i] = fmt.Sprintf("feat-%d", i)
	}
	blocks := make([]byte, 32)
	binary.LittleEndian.PutUint32(blocks[16:20], 0x00000001) // bit 0 of block 1 -> feat-32
	features := parseFeatureBlocks(blocks, names)
	want := []string{"feat-32"}
	if !reflect.DeepEqual(features.Hardware, want) {
		t.Fatalf("Hardware = %#v, want %#v", features.Hardware, want)
	}
}

func TestParseFeatureBlocksWithoutNames(t *testing.T) {
	blocks := make([]byte, 16)
	binary.LittleEndian.PutUint32(blocks[0:4], 0xffffffff)
	features := parseFeatureBlocks(blocks, nil)
	if len(features.Hardware) != 0 || len(features.Active) != 0 {
		t.Fatalf("expected no features without names, got %#v", features)
	}
}

func TestRSSHashFuncName(t *testing.T) {
	cases := map[byte]string{
		0x01: "toeplitz",
		0x02: "xor",
		0x04: "crc32",
		0x00: "0x00",
		0x03: "0x03",
	}
	for hfunc, want := range cases {
		if got := rssHashFuncName(hfunc); got != want {
			t.Fatalf("rssHashFuncName(0x%x) = %q, want %q", hfunc, got, want)
		}
	}
}

func TestEthtoolPortName(t *testing.T) {
	cases := map[byte]string{
		0x00: "twisted-pair",
		0x01: "aui",
		0x02: "mii",
		0x03: "fibre",
		0x04: "bnc",
		0x05: "direct-attach",
		0xff: "none",
		0x7f: "0x7f",
	}
	for port, want := range cases {
		if got := ethtoolPortName(port); got != want {
			t.Fatalf("ethtoolPortName(0x%x) = %q, want %q", port, got, want)
		}
	}
}

func TestEthtoolTransceiverAndMDIXName(t *testing.T) {
	if got := ethtoolTransceiverName(0x00); got != "internal" {
		t.Fatalf("transceiver internal = %q", got)
	}
	if got := ethtoolTransceiverName(0x01); got != "external" {
		t.Fatalf("transceiver external = %q", got)
	}
	if got := ethtoolTransceiverName(0x09); got != "0x09" {
		t.Fatalf("transceiver unknown = %q", got)
	}
	if got := ethtoolMDIXName(0x02); got != "mdi-x" {
		t.Fatalf("mdix = %q", got)
	}
	if got := ethtoolMDIXName(0x03); got != "auto" {
		t.Fatalf("mdix auto = %q", got)
	}
	if got := ethtoolMDIXName(0x05); got != "0x05" {
		t.Fatalf("mdix unknown = %q", got)
	}
}
