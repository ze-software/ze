// Design: docs/architecture/mpls/mpls-kernel.md -- a label entry no writer installed
// Related: doctor.go -- CodeDoctorNoWriter and its explanation

package mplsfib

import "github.com/ze-software/ze/internal/core/diagnostic"

// init registers the explanation of CodeDoctorNoWriter, so `ze show diagnostic`
// describes the code every label source's doctor check reports.
func init() {
	if err := diagnostic.Register(noWriterCodeMeta); err != nil {
		panic("BUG: mplsfib: doctor code registration refused: " + err.Error())
	}
}
