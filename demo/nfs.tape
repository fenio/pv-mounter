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
Set Env TERM "xterm-256color"
Set Env LS_COLORS "di=1;34:ln=1;36:so=1;35:pi=33:ex=1;32:bd=1;33:cd=1;33:su=37;41:sg=30;43:tw=30;42:ow=34;42"
Output ../nfs.gif
Sleep 3s
Type "kubectl get pvc"
Enter
Wait
Sleep 1s
Type "kubectl get pod"
Enter
Wait
Sleep 1s
Type "sudo kubectl pv-mounter mount --backend nfs default unmounted-pvc foo"
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
Type "sudo kubectl pv-mounter clean --backend nfs default unmounted-pvc foo"
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
