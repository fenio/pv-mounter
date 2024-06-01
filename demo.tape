Set Shell zsh
Set FontSize 16
Set Width 1600
Set Height 600
Set TypingSpeed 180ms
Require kubectl
Output demo.gif
Sleep 3s
Type "kubectl get pvc"
Enter
Sleep 1s
Type "kubectl get pod"
Enter
Sleep 1s
Type "ls foo"
Enter
Sleep 1s
Type "kubectl pv-mounter mount demo unmounted-pvc foo"
Enter
Sleep 20s
Type "kubectl get pod"
Enter
Sleep 1s
Type "ls foo"
Enter
Sleep 2s
Type "kubectl pv-mounter clean demo unmounted-pvc foo"
Enter
Sleep 5s
Type "kubectl get pod"
Enter
Sleep 1s
Type "ls foo"
Enter
Sleep 1s
Ctrl+D

