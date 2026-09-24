// A bare gated marker resolves in the unit's own package: own/ declares the
// test it names, so the table is gated.
package own

// enumeration: gated by TestSpeedsMatchTheModel
var speeds = []string{"9600", "19200", "38400", "57600", "115200"}
