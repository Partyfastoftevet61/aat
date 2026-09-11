# The hero recording: one plan takes a shop order through every state.
# Run by demos/run.sh, which starts the sandbox, extracts the shop project into
# ./shop, and puts aat on PATH. Writes out/demo-plan.gif and out/demo-plan.mp4.

Output out/demo-plan.gif
Output out/demo-plan.mp4

Require aat

Set Shell "bash"
Set Theme "Catppuccin Mocha"
Set FontFamily "JetBrains Mono"
Set FontSize 15
Set Width 1100
Set Height 760
Set Padding 20
Set Framerate 24
Set TypingSpeed 45ms
Set CursorBlink false

Hide
Type "cd shop && clear"
Enter
Wait
Show

Sleep 500ms
Type "aat run plan full-lifecycle"
Sleep 400ms
Enter
Wait@60s
Sleep 4s
