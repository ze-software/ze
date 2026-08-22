//go:build race

package ssh

// answerFrameRaceOverhead is what the race detector adds to the allocation
// count of one exec-channel frame, measured on 2026-08-22.
//
// It exists because the detector's cost belongs to the detector rather than to
// the frame, and a ceiling raised for both passes would stop the pass that runs
// over the whole tree from seeing a lost buffer at all.
const answerFrameRaceOverhead = 2
