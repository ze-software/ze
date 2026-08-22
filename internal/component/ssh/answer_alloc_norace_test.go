//go:build !race

package ssh

// answerFrameRaceOverhead is nothing without the race detector: the ceiling in
// answer_test.go is the count measured on this build, so a buffer the frame
// allocates or fails to return reads as one more. Its sibling
// (answer_alloc_race_test.go) carries what the detector adds.
const answerFrameRaceOverhead = 0
