// TestZZSizes was a throwaway diagnostic created during the ext-10 integration
// to measure struct sizes/offsets; it asserted nothing (only t.Logf). The permanent size
// guardrail is TestInterfaceConfigCopyBudget in zz_size_test.go. This file is safe to delete
// (recorded in tmp/delete-*.sh).
package ospf
