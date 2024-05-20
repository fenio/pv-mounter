# pv-mounter 

A tool to locally mount k8s PVCs using sshfs.

Might be used as `kubectl` plugin.

## Quick Start

```
kubectl krew install pv-mounter

kubectl pv-mounter mount <namespace> <pvc-name> <local-mountpoint>
kubectl pv-mounter clean <namespace> <pvc-name> <local-mountpoint>

```


