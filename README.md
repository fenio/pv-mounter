# pv-mounter

[![build](https://github.com/fenio/pv-mounter/actions/workflows/build.yaml/badge.svg)](https://github.com/fenio/pv-mounter/actions/workflows/build.yaml)
[![test](https://github.com/fenio/pv-mounter/actions/workflows/test.yaml/badge.svg)](https://github.com/fenio/pv-mounter/actions/workflows/test.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/fenio/pv-mounter)](https://goreportcard.com/report/github.com/fenio/pv-mounter)
![Latest GitHub release](https://img.shields.io/github/release/fenio/pv-mounter.svg)
[![GitHub license](https://img.shields.io/github/license/fenio/pv-mounter)](https://github.com/fenio/pv-mounter/blob/main/LICENSE)
![GitHub stars](https://img.shields.io/github/stars/fenio/pv-mounter.svg?label=github%20stars)
[![GitHub issues](https://img.shields.io/github/issues/fenio/pv-mounter)](https://github.com/fenio/pv-mounter/issues)
![GitHub all releases](https://img.shields.io/github/downloads/fenio/pv-mounter/total)
![Docker Pulls](https://img.shields.io/docker/pulls/bfenski/volume-exposer?label=volume-exposer%20-%20docker%20pulls)
![Docker Pulls](https://img.shields.io/docker/pulls/bfenski/nfs-ganesha?label=nfs-ganesha%20-%20docker%20pulls)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/fenio/pv-mounter/badge)](https://scorecard.dev/viewer/?uri=github.com/fenio/pv-mounter)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/9551/badge)](https://www.bestpractices.dev/projects/9551)
[![codecov](https://codecov.io/gh/fenio/pv-mounter/graph/badge.svg?token=DHYZ71SVDV)](https://codecov.io/gh/fenio/pv-mounter)

A tool to locally mount Kubernetes Persistent Volumes (PVs) using SSHFS or NFS.

This tool can also be used as a kubectl plugin.

<details>
  <summary><h2 style="display: inline-block; margin: 0;">Disclaimer</h2></summary>

This tool was created with significant help from [ChatGPT-4o](https://chatgpt.com/?model=gpt-4o) and [perplexity](https://www.perplexity.ai/).
In fact, I didn't have to write much of the code myself, but I spent a lot of time crafting the correct prompts for these tools.

**Update**

The above was true for versions 0.0.x. With version 0.5.0, I actually had to learn some Go. While I still used help from GPT, I had to completely change my approach.
AI alone wasn't able to create fully functional code that met all my requirements.

I published it using the Apache-2.0 license because the initial [repository](https://github.com/replicatedhq/krew-plugin-template) was licensed this way. However, to be honest, I'm not sure how such copy-and-paste code should be licensed.

</details>

## Rationale

I often need to copy some files from my [homelab](https://github.com/fenio/homelab) which is running on Kubernetes.
Having the ability to work on these files locally greatly simplifies this task. Thus, pv-mounter was born to automate that process.

## What exactly does it do?

pv-mounter supports two backends: **SSH** (default) and **NFS** (`--backend nfs`). Both work the same way depending on the volume's access mode.

For RWX (ReadWriteMany) volumes or unmounted RWO (ReadWriteOnce) volumes:

* Spawns a standalone Pod with the chosen backend (SSH daemon or NFS-Ganesha) and binds it to the existing PVC.
* Creates a port-forward to make it locally accessible.
* Mounts the volume locally using SSHFS or NFSv4.

For already mounted RWO volumes:

* Creates an ephemeral container with the chosen backend within the Pod that currently mounts the volume.
* Creates a port-forward directly to the workload Pod to make it locally accessible.
* Mounts the volume locally using SSHFS or NFSv4.

## Prerequisites

* **SSH backend (default):** You need a working SSHFS setup.
  * Instructions for [macOS](https://osxfuse.github.io/).
  * Instructions for [Linux](https://github.com/libfuse/sshfs).
* **NFS backend (`--backend nfs`):** You need an NFS client.
  * macOS: Built-in, no installation needed.
  * Linux: `sudo apt install nfs-common` (Debian/Ubuntu) or `sudo dnf install nfs-utils` (RHEL/Fedora).

## Quick Start / Usage

```
kubectl krew install pv-mounter

kubectl pv-mounter mount [--backend ssh|nfs] [--needs-root] [--debug] [--image] [--image-secret] <namespace> <pvc-name> <local-mount-point>
kubectl pv-mounter clean [--backend ssh|nfs] <namespace> <pvc-name> <local-mount-point>

```

Obviously, you need to have working [krew](https://krew.sigs.k8s.io/docs/user-guide/setup/install/) installation first.

Or you can simply grab binaries from [releases](https://github.com/fenio/pv-mounter/releases).

## Security

I spent quite some time making the solution as secure as possible.

### SSH backend

* SSH keys used for connections between various components are generated every time from scratch and once you wipe the environment clean, you won't be able to connect back into it using the same credentials.
* Containers / Pods are using minimal possible privileges:

```
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
runAsUser = XYZ
runAsGroup = XYZ
runAsNonRoot = true
```

sshd_config is also limited as much as possible:

```
PermitRootLogin no
PasswordAuthentication no
```

Above, it's not true if you're using the --needs-root option or the NEEDS_ROOT environment variable, but well, you've asked for it.

### NFS backend

* NFS-Ganesha requires more capabilities than SSH (`SYS_ADMIN`, `DAC_READ_SEARCH`, `DAC_OVERRIDE`, `SYS_RESOURCE`, `CHOWN`, `FOWNER`, `SETUID`, `SETGID`) and runs with `seccompProfile: Unconfined`.
* Standalone NFS pods run as root. Ephemeral NFS containers run as the workload pod's UID.
* NFS exports use `No_Root_Squash` and `SecType = sys` (AUTH_SYS) — this is acceptable since traffic stays within the kubectl port-forward tunnel (localhost only).

## Limitations

The tool has a "clean" option that does its best to clean up all the resources it created for mounting the volume locally.
However, ephemeral containers can't be removed or deleted. That's the way Kubernetes works.
As part of the cleanup, this tool kills the process that keeps its ephemeral container alive.
I confirmed it also kills other processes that were running in that container, but the container itself remains in a limbo state.

## Windows

Since I can't test Windows binaries, they are not included. However, since there seems to exist a working Windows implementation of SSHFS, in theory it should work.

## Demos

Created with [VHS](https://github.com/charmbracelet/vhs) tool.

The demo GIFs are automatically regenerated when code or tape files change.

### SSH

#### RWX or unmounted RWO volume

![ssh-rwx](ssh-rwx.gif)

#### Mounted RWO volume

![ssh-rwo](ssh-rwo.gif)

### NFS

#### RWX or unmounted RWO volume

![nfs-rwx](nfs-rwx.gif)

#### Mounted RWO volume

![nfs-rwo](nfs-rwo.gif)

## FAQ

Ask more questions, if you like ;)

#### I need to run the mounter pod as root, but my [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) blocks the creation. What needs to be done?

You can add a label to the namespace you want the pod to be spawned in, to create an exception.

`kubectl label namespace NAMESPACE-NAME pod-security.kubernetes.io/enforce=privileged`

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=fenio/pv-mounter&type=Date)](https://star-history.com/#fenio/pv-mounter&Date)
