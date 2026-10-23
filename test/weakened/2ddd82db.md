| Test | Reason |
|---|---|
| TestSplitCommand | Renamed TestRunArgv with splitCommand, the function it called, which became runArgv when the run line stopped splitting on whitespace alone. Its whitespace case is TestRunArgv's first case, and TestRunArgv adds quoted words, an apostrophe, the blank refusal and the unclosed-quote refusal. |
| internal_test | The fatal count fell by one because TestSplitCommand's separate length check became one slices.Equal comparison per case in TestRunArgv; every case it asserted is still asserted, and more cases were added. |
