<!--
The title of this pull request becomes a line in the release notes, which are
generated from merged pull requests. Write it as a Conventional Commit a user
can read: "fix(cli): pull no longer drops local edits on a conflict", not
"fix stuff" or "address review".
-->

## What and why

<!-- What changes for a user of cerne, and why. Link the issue it closes: "Closes #123". -->

## How it was tested

<!-- The test that fails without this change, and how you ran it. -->

## Checklist

- [ ] A test that fails without this change
- [ ] One concern per commit, Conventional Commits messages
- [ ] Touches `ui/src`? Say so here: a maintainer rebuilds `ui/dist` (do not commit it)
- [ ] Does not touch `docs.lock`
