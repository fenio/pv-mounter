# pv-mounter 

A tool to locally mount k8s PVs using SSHFS.

Might be used as `kubectl` plugin.

## Disclaimer

This tool was created with huge help of [ChatGPT-4o](https://chatgpt.com/?model=gpt-4o) and [perplexity](https://www.perplexity.ai/).
In fact I didn't have to write my own code almost at all but I had to spend a lot of time writing correct prompts for these tools.
I published it using Apache-2.0 license cause initial [repository](https://github.com/replicatedhq/krew-plugin-template) was licensed this way but to be honest I'm not sure how such copy&paste stuff should be licensed.

## Rationale

I often need to copy some files from my [homelab](https://github.com/fluxcd/flux2) which is running on k8s. Having ability to work on these files locally greatly simplifies this task.
Thus pv-mounter was born to automate that process.

## What exactly does it do?

Few things. Namely:

* spawns POD with minimalistic image that contains SSH daemon and binds to existing PVC
* creates port-forward to make it locally accessible
* mounts volume locally using SSHFS 

See also demo below.

## Prerequisities

* You need working SSHFS setup.

## Quick Start

```
kubectl krew install pv-mounter

kubectl pv-mounter mount <namespace> <pvc-name> <local-mountpoint>
kubectl pv-mounter clean <namespace> <pvc-name> <local-mountpoint>

```

In the meantime you can simply grab binaries from [releases](https://github.com/fenio/pv-mounter/releases).

## Demo

![Demo](demo.gif)

## Limitations

* It's not possible to mount PVC with RWO access mode if it's already mounted somewhere else

I tried to workaround it by using ephemeral container but unfortunately they're too limited for that task as they can't expose port thus it's not possible to expose SSH port.

## FAQ

Ask questions first ;)
