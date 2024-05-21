# pv-mounter 

A tool to locally mount k8s PVs using sshfs.

Might be used as `kubectl` plugin.

## Disclaimer

This tool was created with help of [ChatGPT-4o](https://chatgpt.com/?model=gpt-4o) and [perplexity](https://www.perplexity.ai/).
I licensed it using Apache-2.0 license cause initial [repository](https://github.com/replicatedhq/krew-plugin-template) was licensed this way but to be honest I'm not sure how copy&paste stuff should be licensed.

## Quick Start

Will start working once https://github.com/kubernetes-sigs/krew-index/pull/3844 is approved.

```
kubectl krew install pv-mounter

kubectl pv-mounter mount <namespace> <pvc-name> <local-mountpoint>
kubectl pv-mounter clean <namespace> <pvc-name> <local-mountpoint>

```

## Demo


![Demo](demo.gif)

