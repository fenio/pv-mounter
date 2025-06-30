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
Output ../unmounted.gif
Sleep 3s
Type "kubectl get pvc"
Enter
Wait
Sleep 1s
Type "kubectl get pod"
Enter
Wait
Sleep 1s
Type "ls foo"
Enter
Wait
Sleep 1s
Type "kubectl pv-mounter mount demo unmounted-pvc foo"
Enter
Wait
Sleep 1s
Type "kubectl get pod"
Enter
Wait
Sleep 1s
Type "ls foo"
Enter
Wait
Sleep 1s
Type "kubectl pv-mounter clean demo unmounted-pvc foo"
Enter
Wait
Sleep 1s
Type "kubectl get pod"
Enter
Wait
Sleep 1s
Type "ls foo"
Enter
Sleep 5s
Ctrl+D
