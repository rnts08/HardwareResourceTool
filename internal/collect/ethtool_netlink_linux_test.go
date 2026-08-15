//go:build linux

package collect

import (
	"reflect"
	"testing"

	"github.com/mdlayher/ethtool"
)

func TestLinkModeNames(t *testing.T) {
	got := linkModeNames([]ethtool.AdvertisedLinkMode{{Name: "10G"}, {}, {Name: "25G"}})
	want := []string{"10G", "25G"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linkModeNames() = %#v, want %#v", got, want)
	}
}

func TestEnrichNetworksUnsupportedNameDoesNotFailCollection(t *testing.T) {
	data := enrichNetworks([]string{"hardware-resources-test-nonexistent"})
	if _, ok := data["hardware-resources-test-nonexistent"]; !ok {
		t.Fatal("enrichNetworks() did not return an entry for the requested interface")
	}
}
