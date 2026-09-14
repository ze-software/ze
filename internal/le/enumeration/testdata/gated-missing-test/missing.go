// A gated marker naming a test no _test.go declares. The marker gates nothing,
// so the literal stays a finding and the marker is reported beside it.
package fixture

// enumeration: gated by TestSpeedsNeverWritten
var speeds = []string{"9600", "19200", "38400", "57600", "115200"}
