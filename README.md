# pv-mounter 

[![build](https://github.com/fenio/pv-mounter/actions/workflows/release.yml/badge.svg)](https://github.com/fenio/pv-mounter/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/fenio/pv-mounter)](https://goreportcard.com/report/github.com/fenio/pv-mounter)
![Latest GitHub release](https://img.shields.io/github/release/fenio/pv-mounter.svg)
[![GitHub license](https://img.shields.io/github/license/fenio/pv-mounter)](https://github.com/fenio/pv-mounter/blob/main/LICENSE)
![GitHub stars](https://img.shields.io/github/stars/fenio/pv-mounter.svg?label=github%20stars)
[![GitHub issues](https://img.shields.io/github/issues/utkuozdemir/pv-migrate)](https://github.com/utkuozdemir/pv-migrate/issues)
[![GitHub issues](https://img.shields.io/github/issues/fenio/pv-mounter)](https://github.com/fenio/pv-mounter/issues)
[[GitHub issues](https://img.shields.io/github/issues/fenio/pv-mounter)
![GitHub all releases](https://img.shields.io/github/downloads/fenio/pv-mounter/total)
![Docker Pulls](https://img.shields.io/docker/pulls/bfenski/volume-exposer?label=volume-exposer%20-%20docker%20pulls)

A tool to locally mount Kubernetes Persistent Volumes (PVs) using SSHFS.

This tool can also be used as a kubectl plugin.

## Disclaimer

This tool was created with significant help from [ChatGPT-4o](https://chatgpt.com/?model=gpt-4o) and [perplexity](https://www.perplexity.ai/).
In fact, I didn't have to write much of the code myself, but I spent a lot of time crafting the correct prompts for these tools.

**Update**

The above was true for versions 0.0.x. With version 0.5.0, I actually had to learn some Go. While I still used help from GPT, I had to completely change my approach. 
It wasn't able to create fully functional code that met all my requirements.

I published it using the Apache-2.0 license because the initial [repository](https://github.com/replicatedhq/krew-plugin-template) was licensed this way. However, to be honest, I'm not sure how such copy-and-paste code should be licensed.

## Rationale

I often need to copy some files from my [homelab](https://github.com/fenio/homelab) which is running on Kubernetes. 
Having the ability to work on these files locally greatly simplifies this task. Thus, pv-mounter was born to automate that process.

## What exactly does it do?

It performs a few tasks. In the case of volumes with RWX (ReadWriteMany) access mode or unmounted RWO (ReadWriteOnce):

* Spawns a POD with a minimalistic image that contains an SSH daemon and binds it to the existing PVC.
* Creates a port-forward to make it locally accessible.
* Mounts the volume locally using SSHFS.

For already mounted RWO volumes, it's a bit more complex:

* Spawns a POD with a minimalistic image that contains an SSH daemon and acts as a proxy to an ephemeral container.
* Creates an ephemeral container within the POD that currently mounts the volume.
* From that ephemeral container, establishes a reverse SSH tunnel to the proxy POD.
* Creates a port-forward to the proxy POD onto the port exposed by the tunnel to make it locally accessible.
* Mounts the volume locally using SSHFS.

See the demo below for more details.

## Prerequisities

* You need a working SSHFS setup.

Instructions for [macOS](https://osxfuse.github.io/).
Instructions for [Linux](https://github.com/libfuse/sshfs).

## Quick Start

```
kubectl krew install pv-mounter

kubectl pv-mounter mount <namespace> <pvc-name> <local-mountpoint>
kubectl pv-mounter clean <namespace> <pvc-name> <local-mountpoint>

```

Obviously, you need to have working [krew](https://krew.sigs.k8s.io/docs/user-guide/setup/install/) installation first.

Or you can simply grab binaries from [releases](https://github.com/fenio/pv-mounter/releases).

## Limitations

The tool has a clean option that does its best to clean up all the resources it created for mounting the volume locally. 
However, ephemeral containers can't be removed or deleted. That's the way Kubernetes works. 
As part of the cleanup, the tool kills the process that keeps the ephemeral container alive. 
I confirmed it also kills other processes that were running in that container, but the container itself remains in a pretty weird state.

## Demo

![Demo](demo.gif)


### Windows

Since I can't test Windows binaries, they are not included. However, I saw there is an SSHFS implementation for Windows, so in theory, this should work.

## FAQ

Ask questions first ;)
