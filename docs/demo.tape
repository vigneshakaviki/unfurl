# Generate the hero GIF:  vhs docs/demo.tape  ->  docs/demo.gif
# Requires: https://github.com/charmbracelet/vhs  (and unfurl built on PATH)
Output docs/demo.gif
Set FontSize 20
Set Width 1200
Set Height 720
Set Theme "Dracula"
Set Padding 20

Type "unfurl ./charts/api --explain"
Sleep 500ms
Enter
Sleep 4s
