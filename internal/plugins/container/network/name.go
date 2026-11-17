// Design: docs/architecture/container/network.md -- device names and hardware addresses
// Related: register.go -- the device name prefix the package grants itself
// Related: verify.go -- the rules that judge a network name

package network

import (
	"crypto/sha256"
	"net"
	"strconv"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// devicePrefix is the device name prefix the package grants itself. The
	// interface component composes a host end as the prefix, a hyphen, the
	// bridge's interface index, a hyphen and the slot, so a host end reads
	// zc-12-0. A bridge name starts with it too.
	devicePrefix = "zc"
	// bridgePrefix starts every bridge name: zcn- plus the network name.
	bridgePrefix = devicePrefix + "n-"
	// networkNameLengthMax is the longest network name: a kernel device name
	// holds 15 bytes, and the bridge prefix takes 4 of them.
	networkNameLengthMax = 11
	// containerLinkPrefix names the container end of each link, followed by
	// the attachment's position in the instance's network list.
	containerLinkPrefix = "eth"
)

// bridgeName answers the bridge device name of network name, or refuses a
// name a device name cannot hold. It never truncates: two networks whose
// names share 11 characters would otherwise get one bridge.
func bridgeName(name string) (string, error) {
	if err := checkNetworkName(name); err != nil {
		return "", err
	}
	return bridgePrefix + name, nil
}

// checkNetworkName refuses a network name outside the form the schema states:
// 1 to 11 characters, lowercase letters, digits and interior hyphens. The
// schema refuses the same names; this is the second check of the pair, for a
// delivery that did not pass that parser.
func checkNetworkName(name string) error {
	if name == "" {
		return refuseNetworkName(name, "it is empty")
	}
	if len(name) > networkNameLengthMax {
		return refuseNetworkName(name, "it is longer than 11 characters, and the bridge device name zcn-NAME holds at most 15")
	}
	if name[0] == '-' {
		return refuseNetworkName(name, "it starts with a hyphen")
	}
	if name[len(name)-1] == '-' {
		return refuseNetworkName(name, "it ends with a hyphen")
	}
	// Bounded by networkNameLengthMax, checked above.
	for i := range len(name) {
		if !isNetworkNameByte(name[i]) {
			return refuseNetworkName(name, "it holds a character other than a lowercase letter, a digit or a hyphen")
		}
	}
	return nil
}

func isNetworkNameByte(c byte) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= '0' && c <= '9' {
		return true
	}
	return c == '-'
}

func refuseNetworkName(name, defect string) error {
	var problem textbuf.Buffer
	problem.Str("container network name ").Quoted(name).Str(" is not a name ze accepts: ").Str(defect)
	return opererr.New(
		problem.String(),
		"the network's bridge device is named zcn- plus the name, and ze refuses a name it would have to shorten",
		"set container network NAME with a name of 1 to 11 lowercase letters, digits and interior hyphens",
	)
}

// containerLinkName answers the name of the container end of the attachment
// at position in the instance's network list: eth0, eth1, and so on.
func containerLinkName(position int) string {
	return containerLinkPrefix + strconv.Itoa(position)
}

// derivedHardwareAddress answers the hardware address of an attachment that
// sets none: six bytes of the SHA-256 of the network and instance names, with
// the locally administered bit set and the multicast bit clear. The same two
// names always give the same address, so a neighbour's entry stays valid
// across a restart, and another container on the address gets another one.
func derivedHardwareAddress(network, instance string) net.HardwareAddr {
	// A zero byte separates the names: neither name holds one, so "ab"+"c" and
	// "a"+"bc" hash differently.
	sum := sha256.Sum256([]byte(network + "\x00" + instance))
	address := net.HardwareAddr(sum[:6])
	// IEEE 802: bit 1 of the first octet marks a locally administered
	// address, and bit 0 a group (multicast) address.
	address[0] = (address[0] | 0x02) &^ 0x01
	return address
}
