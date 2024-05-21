# pv-mounter 

A tool to locally mount k8s PVs using sshfs.

Might be used as `kubectl` plugin.

## Quick Start

Will start working once https://github.com/kubernetes-sigs/krew-index/pull/3844 is approved.

```
kubectl krew install pv-mounter

kubectl pv-mounter mount <namespace> <pvc-name> <local-mountpoint>
kubectl pv-mounter clean <namespace> <pvc-name> <local-mountpoint>

```

## Demo


![Demo](demo.gif)

