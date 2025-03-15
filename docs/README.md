# PV Mounter Documentation

A Kubernetes plugin for direct PVC access through SSHFS mounting.
		
Examples:
  # Mount a PVC
  kubectl pv-mounter mount my-namespace my-pvc ./local-mount
  
  # Clean up a mounted PVC
  kubectl pv-mounter clean my-namespace my-pvc ./local-mount
## Command Reference

```
Usage:
  pv-mounter [command]

Available Commands:
  clean       Clean the mounted PVC
  help        Help about any command
  mount       Mount a PVC to a local directory

Flags:
  -h, --help   help for pv-mounter

Use "pv-mounter [command] --help" for more information about a command.
```

## Detailed Commands

- [pv-mounter clean](commands/pv-mounter_clean.md)
- [pv-mounter help](commands/pv-mounter_help.md)
- [pv-mounter mount](commands/pv-mounter_mount.md)
