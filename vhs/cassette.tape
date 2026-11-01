# Drives past through its main screens for the README gif + screenshots.
# Run via `make vhs`, which records it inside a container (see vhs/Dockerfile).
# /out is the host-mounted assets dir; gif lands in gifs/, screenshots in screenshots/.
Output "/out/gifs/past.gif"

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
Screenshot "/out/screenshots/browse.png"

# open the first entry and reveal its password, then mask it again
Down Sleep 1.5s
Enter Sleep 2.5s
Screenshot "/out/screenshots/reveal.png"
Enter Sleep 1s

# fold a folder away, then open it again
Up Sleep 800ms
Left Sleep 1.5s
Right Sleep 1s

# narrow the tree down with the filter
Type "/" Sleep 500ms
Type "mail" Sleep 1s
Enter Sleep 2s
Escape Sleep 1s

# the add-login form
Type "g" Sleep 800ms
Type "work/jira" Sleep 2s
Screenshot "/out/screenshots/add.png"
Escape Sleep 1s

# help overlay
Type "?" Sleep 2.5s
Screenshot "/out/screenshots/help.png"
Type "?" Sleep 1s

Sleep 1.5s
Type "q"
Sleep 1s
