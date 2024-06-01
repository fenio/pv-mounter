Set Shell zsh
Set FontSize 18
Set Width 1600
Set Height 600
Require kubectl
Output demo.gif
Sleep 3s
Type "kubectl get pvc"
Enter
Sleep 500ms
Type "kubectl get pod
Enter
Sleep 500ms
Type "kubectl pv-mounter demo unmounted-pvc foo"
Enter
Sleep 500ms
Type "kubectl get pod"
Enter
Sleep 500ms
Type "ls foo"
Enter
Sleep 1s
Ctrl+D

