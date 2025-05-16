
## Usage
The following assumes you have the plugin installed via

```shell
kubectl krew install pv-mounter
```

### Mount PVC to local directory

```shell
kubectl pv-mounter mount some-ns some-pvc some-mountpoint
```

You can optionally set a CPU limit for the helper pod:

```shell
kubectl pv-mounter mount --cpu-limit 200m some-ns some-pvc some-mountpoint
```

Or use the environment variable:

```shell
CPU_LIMIT=200m kubectl pv-mounter mount some-ns some-pvc some-mountpoint
```

### Unmount / clean stuff

```shell
kubectl pv-mounter clean some-ns some-pvc some-mountpoint
```

#### Flags and Environment Variables

- `--cpu-limit` (or `CPU_LIMIT`): Set CPU limit for the helper pod (e.g., `200m`).
- `--needs-root` (or `NEEDS_ROOT`): Mount with root privileges.
- `--debug` (or `DEBUG`): Enable debug mode.
- `--image` (or `IMAGE`): Custom container image.
- `--image-secret` (or `IMAGE_SECRET`): Secret for private registry.

## How it works

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
