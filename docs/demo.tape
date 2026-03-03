# taux demo — VHS tape file
# Prerequisites: bash docs/demo-setup.sh
# Usage: vhs docs/demo.tape

Output docs/demo.gif
Set Theme "Catppuccin Mocha"
Set FontSize 14
Set Width 1200
Set Height 700
Set Padding 10
Set Shell zsh

# Hidden setup: isolate from real sessions
Hide
Type "export HOME=/tmp/taux-demo"
Enter
Type "export PATH=$HOME/.local/bin:$PATH"
Enter
Type "export PS1='❯ '"
Enter
Type "clear"
Enter
Show
Sleep 500ms

# 1. CLI stats — show cost tracking right away
Type "taux get stats"
Enter
Sleep 3.5s

# 2. Launch interactive dashboard
Type "taux dashboard"
Enter
Sleep 2s

# 3. Navigate sessions (j/k movement)
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 400ms
Up
Up
Sleep 800ms

# 4. View session detail (tokens, tools, branch)
Enter
Sleep 2.5s
Escape
Sleep 800ms

# 5. Stats tab — cost tracking per period + model
Tab
Sleep 3s

# 6. Projects tab — per-project aggregation
Tab
Sleep 2.5s

# 7. Back to Sessions
Tab
Sleep 800ms

# 8. Search by alias
Type "/"
Sleep 300ms
Type "redis"
Sleep 1.5s
Escape
Sleep 600ms

# 9. Search by model
Type "/"
Sleep 300ms
Type "opus"
Sleep 1.5s
Escape
Sleep 800ms

# 10. Quit
Type "q"
Sleep 500ms
