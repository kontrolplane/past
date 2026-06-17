# Drives past through its main screens for the README gif + screenshots.
# Run via `make vhs`, which records it inside a container (see vhs/Dockerfile).
# /out is the host-mounted assets dir; gif lands in gifs/, screenshots in screenshots/.
Output "/out/gifs/past.gif"

Set Shell bash
Set FontSize 16
Set Width 1920
Set Height 1080

Type "past" Enter
Sleep 4s
Screenshot "/out/screenshots/browse.png"

# reveal the first leaf's password, then mask it again
Down Sleep 1s
Enter Sleep 2.5s
Screenshot "/out/screenshots/reveal.png"
Enter Sleep 1s

# the add-login form
Type "g" Sleep 2.5s
Screenshot "/out/screenshots/add.png"
Escape Sleep 1s

# help overlay
Type "?" Sleep 2.5s
Screenshot "/out/screenshots/help.png"
Type "?" Sleep 1s

Sleep 1.5s
Type "q"
Sleep 1s
