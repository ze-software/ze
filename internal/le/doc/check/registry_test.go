package doccheck

// The retired-name gate's fixtures cite `le test unit all` as the new form of
// a retired one, and the same sweep judges every citation against what le
// registers. The test binary links only the areas its package imports, so the
// `test unit` area registers here: a fixture citing a command le holds is then
// held in the test binary as it is in le.
import _ "github.com/ze-software/ze/internal/le/test/unit"
