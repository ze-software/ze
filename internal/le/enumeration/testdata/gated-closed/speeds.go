// A table holding every value of one YANG enumeration, gated by the agreement
// test the marker names. The Go side carries the fact the model does not (the
// baud rate a serial driver takes), so the two owe each other agreement.
package fixture

// enumeration: gated by TestSpeedsMatchTheModel
var speeds = []string{"9600", "19200", "38400", "57600", "115200"}
