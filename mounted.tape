Require kubectl
Set Shell zsh
Set FontSize 16
Set Padding 10
Set LineHeight 1.2
Set Width 1500
Set Height 600
Set TypingSpeed 180ms
Set WindowBar Colorful
Output mounted.gif
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
Sleep 10s
Ctrl+D

