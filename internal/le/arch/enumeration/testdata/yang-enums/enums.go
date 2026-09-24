// A slice restating EVERY value of one YANG enumeration. A transcription takes
// the whole set; a literal sharing two of its words shares two words.
package fixture

var speeds = []string{"9600", "19200", "38400", "57600", "115200"}

// Two of the same enumeration's five values, which is what a coincidence looks
// like: no finding.
var fastOnly = []string{"57600", "115200"}
