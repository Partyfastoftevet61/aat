# The matrix recording: seven plans across two layer groups, deduplicated and
# run four at a time. Run by demos/run.sh after plan.tape, against the same
# sandbox and ./shop project. Writes out/demo-batch.gif.

Output out/demo-batch.gif

Require aat

Set Shell "bash"
Set Theme "Catppuccin Mocha"
Set FontFamily "JetBrains Mono"
Set FontSize 15
Set Width 1280
Set Height 760
Set Padding 20
Set Framerate 24
Set TypingSpeed 20ms
Set CursorBlink false

Hide
Type "cd shop && clear"
Enter
Wait
Show

Sleep 500ms
Type "aat run batch --layer-group shipping-standard,shipping-express --layer-group basket-gear,basket-apparel --parallel 4"
Sleep 400ms
Enter
Wait@120s
Sleep 4s
