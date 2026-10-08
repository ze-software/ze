# Tree-wide tools never touch another session's files

Several sessions edit this checkout at once. An agent MUST NOT run a
formatter, codemod or rewrite tool (`gofmt -w`, `goimports -w`, `gopls rename`,
`sed -i`, an AST rewriter) over a file set chosen by "every dirty file", a
directory glob, or the whole tree. Pass it only the files the agent itself
wrote or edited in this task. A file another session has uncommitted changes in
is theirs, even when the change would be formatting only.

Origin: on 2026-10-08 an agent ran `gofmt -w` over every dirty Go file and
reformatted four files holding another session's uncommitted BGP work.
