// A spelled gated marker naming a package that declares no such test. The
// marker gates nothing, whatever other packages declare.
package wrong

// enumeration: gated by nowhere:TestSpeedsMatchTheModel
var levels = []string{"9600", "19200", "38400", "57600", "115200"}
