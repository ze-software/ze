//go:build race

package plugin

// answerAllocRaceOverhead is what the race detector adds to the allocation
// count of one answer. It was measured at six on 2026-08-22, over the six exit
// paths TestWriteAnswerReleasesBufferOnError drives and the two answers
// TestWriteAnswerUsesPooledBuffer writes. It was two while an answer held one
// pooled buffer, and it rose when the answer took a second one for the rows it
// holds until the threshold decides its type.
//
// It is seven since 2026-10-12. Under -race, sync.Pool.Put drops a random
// quarter of what it is handed (src/sync/pool.go, "Randomly drop x on
// floor"), so the count a pooled answer reads is a mean over random drops
// rather than a fixed number. The row-arity exit path read 27 on most runs
// and 28 on one, a ceiling of 21 plus six, so six left no room for that
// variance. The pass without -race still reads the exact 21.
//
// It exists because the detector's cost belongs to the detector rather than to
// the writer, and a ceiling raised for both passes would stop the pass that
// runs over the whole tree from seeing a lost buffer at all. The full pass of
// ze-precommit-verify runs without -race and reads the exact number; the
// changed-group pass runs with it and gets this slack.
const answerAllocRaceOverhead = 7
