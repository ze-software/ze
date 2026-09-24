// A table holding every value of one YANG enumeration AND two words that are
// also plugin names. The gated marker does its job on the enumeration; the
// registry copy is a finding whatever the marker says.
package fixture

// enumeration: gated by TestSpeedsMatchTheModel
var table = []string{"9600", "19200", "38400", "57600", "115200", "ospf", "vrrp"}
