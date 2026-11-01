# Drives past through its pages for the README gif and screenshots.
# Run via `make vhs`, which records it inside a container (see vhs/Dockerfile).
# /out is the host-mounted assets dir: the gif lands at its root, every page
# under pages/.
Output "/out/cassette.gif"

Set Shell bash
Set FontSize 20
Set Width 2000
Set Height 1000
Set BorderRadius 10
Set TypingSpeed 100ms
Env COLORTERM truecolor

Hide
Type "past" Enter
Sleep 3s
Show

Sleep 2s
Screenshot "/out/pages/store/overview.png"

# open the first entry and reveal its password, then mask it again
Down Sleep 1.5s
Enter Sleep 2.5s
Screenshot "/out/pages/entry/details.png"
Enter Sleep 1s

# pick a field to copy
Type "c" Sleep 2s
Screenshot "/out/pages/entry/copy.png"
Escape Sleep 1s

# the delete confirmation, cancelled
Type "d" Sleep 2s
Screenshot "/out/pages/entry/delete.png"
Escape Sleep 1s

# fold a folder away, then open it again
Up Sleep 800ms
Left Sleep 1.5s
Right Sleep 1s

# narrow the tree down with the filter
Type "/" Sleep 500ms
Type "mail" Sleep 1s
Enter Sleep 2s
Screenshot "/out/pages/store/filter.png"
Escape Sleep 1s

# the add-login form
Type "g" Sleep 800ms
Type "work/jira" Sleep 2s
Screenshot "/out/pages/entry/create.png"
Escape Sleep 1s

# the github menu
Type "h" Sleep 2s
Screenshot "/out/pages/store/github.png"
Escape Sleep 1s

# help overlay
Type "?" Sleep 2.5s
Screenshot "/out/pages/store/help.png"
Type "?" Sleep 1s

Sleep 1.5s
Type "q"
Sleep 1s
