# pv-mounter 

A `kubectl` plugin to mount PVC locally using sshfs.

## Quick Start

```
kubectl krew install pv-mounter

kubectl pv-mounter mount <namespace> <pvc-name> <local-mountpoint>
kubectl pv-mounter clean <namespace> <pvc-name> <local-mountpoint>

```


