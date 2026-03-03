# taux demo — VHS tape file
# Usage: vhs docs/demo.tape

Output docs/demo.gif
Set Theme "Catppuccin Mocha"
Set FontSize 14
Set Width 1200
Set Height 700
Set Padding 10
Set Shell zsh

# Launch dashboard directly (skip tmux wrapper)
Type "~/.local/bin/taux dashboard"
Enter
Sleep 3s

# Navigate sessions — browse through active/inactive
Down
Sleep 600ms
Down
Sleep 600ms
Down
Sleep 600ms
Down
Sleep 600ms
Up
Up
Sleep 800ms

# View session detail
Enter
Sleep 2.5s
Escape
Sleep 1s

# Switch to Stats tab
Tab
Sleep 2.5s

# Switch to Projects tab
Tab
Sleep 2.5s

# Back to Sessions
Tab
Sleep 1s

# Filter search by alias
Type "/"
Sleep 500ms
Type "redis"
Sleep 1.5s
Escape
Sleep 800ms

# Filter search by project
Type "/"
Sleep 500ms
Type "ml-pipe"
Sleep 1.5s
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
