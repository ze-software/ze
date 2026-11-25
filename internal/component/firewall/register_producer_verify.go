// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- original-tree validator registration for producer firewall tables
// Related: producer_verify.go -- verifyProducerTables implementation

package firewall

import "github.com/ze-software/ze/internal/component/config"

// init registers the firewall-producer-tables original-tree validator so boot,
// reload, commit and `ze config validate` all verify producer-built tables
// against the candidate firewall backend.
func init() {
	if err := config.RegisterTreeValidator(producerTablesValidatorName, verifyProducerTables); err != nil {
		panic("BUG: firewall: producer-tables tree validator registration failed: " + err.Error())
	}
}
