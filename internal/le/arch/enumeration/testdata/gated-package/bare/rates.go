// A bare gated marker naming a test that two OTHER packages declare and this
// one does not. The name alone resolves nowhere in bare/, so the marker gates
// nothing: the table stays a finding and the marker is reported beside it.
package bare

// enumeration: gated by TestSpeedsMatchTheModel
var rates = []string{"9600", "19200", "38400", "57600", "115200"}
