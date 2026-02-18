Require kubectl
Set Shell zsh
Set FontSize 16
Set Padding 10
Set LineHeight 1.2
Set Width 1500
Set Height 600
Set TypingSpeed 180ms
Set WindowBar Colorful
Set WaitTimeout 30s
Env TERM "xterm-256color"
Env LS_COLORS "di=1;38;5;75:ln=38;5;44:so=38;5;205:pi=38;5;215:ex=1;38;5;114:bd=38;5;215:cd=38;5;215"
Output ../nfs-mounted.gif
Sleep 3s
Type "kubectl get pvc"
Enter
Wait
Sleep 1s
Type "kubectl get pod"
Enter
Wait
Sleep 1s
Type "sudo kubectl pv-mounter mount --backend nfs default mounted-pvc foo"
Enter
Wait
Sleep 1s
Type "kubectl get pod"
Enter
Wait
Sleep 1s
Type "ls --color foo"
Enter
Wait
Sleep 1s
Type "sudo kubectl pv-mounter clean --backend nfs default mounted-pvc foo"
Enter
Wait
Sleep 1s
Type "kubectl get pod"
Enter
Wait
Sleep 1s
Type "ls --color foo"
Enter
Wait
Sleep 5s
Ctrl+D
