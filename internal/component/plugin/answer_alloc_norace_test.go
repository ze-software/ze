//go:build !race

package plugin

// answerAllocRaceOverhead is nothing without the race detector: the ceilings in
// dispatch_test.go are the counts measured on this build, so a buffer an answer
// allocates or fails to return reads as one more. Its sibling
// (answer_alloc_race_test.go) carries what the detector adds.
const answerAllocRaceOverhead = 0
