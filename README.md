# booter

A tool to easily boot Talos machines using PXE.

## Usage

Run `booter` in a container on the host network:

```bash
docker run --rm --network host \
  ghcr.io/siderolabs/booter:v0.1.0
```

Then, power on machines in the **same subnet** as `booter` with **PXE boot** enabled (both UEFI and legacy BIOS are supported).  
Recommended boot order: **disk first, then network**.

To connect the machines to **Omni**, go to the **Omni Overview** page, click **“Copy Kernel Parameters”**, and run `booter` with the copied arguments:

```bash
docker run --rm --network host \
  ghcr.io/siderolabs/booter:v0.1.0 \
  <KERNEL_ARGS>
```

### Using Omni to manage boot assets

Alternatively, `booter` can connect to the Omni API directly.
In this mode:

- machines are connected to Omni without any kernel arguments to copy,
- boot assets are resolved through Omni, from the image factory Omni is configured with.
  This is required for an **Enterprise Image Factory**, which requires authentication on its boot assets:
  Omni authenticates against the factory and hands out iPXE URLs that carry a short-lived download token, so `booter` needs no factory credentials of its own,
- the default Talos version is the one Omni uses as its default.

Create an Omni service account with the **Operator** role, and pass its key in the `OMNI_SERVICE_ACCOUNT_KEY` environment variable:

```bash
docker run --rm --network host \
  -e OMNI_SERVICE_ACCOUNT_KEY=<SERVICE_ACCOUNT_KEY> \
  ghcr.io/siderolabs/booter:v0.1.0 \
  --omni-api-endpoint https://<ACCOUNT>.omni.siderolabs.io
```

The `--image-factory-base-url` and `--image-factory-pxe-base-url` flags cannot be used in this mode.

To see more options:

```bash
docker run --rm --network host ghcr.io/siderolabs/booter:v0.1.0 --help
```

## Development

`booter` also builds and runs natively without docker.
On a fresh clone, fetch the iPXE binaries which get embedded into the binary once, then regular Go commands work:

```bash
make fetch-source-assets
go build ./...
```
