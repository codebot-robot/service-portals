# service portals

Sevice Portals are simple HTTP/HTTPS proxy servers that run inside a kubernetes cluster, and make it easier to consume services that run outside the cluster.

## Accelerating Docker Build

When building Docker images, you often download large artifacts (e.g. `pip install -r requirements.txt`). You can speed up these downloads by using the `artifact-portal` as a caching proxy.

To use it:

1. Run the `artifact-portal` proxy:
   ```sh
   go run ./cmd/artifact-portal/main.go
   ```
   By default, it listens on port 8080 and caches files in `/tmp/artifact-cache`.

2. Build your Docker image and pass the proxy using `--build-arg`:
   ```sh
   docker build \
     --build-arg http_proxy=http://host.docker.internal:8080 \
     --build-arg https_proxy=http://host.docker.internal:8080 \
     -t my-image .
   ```

Make sure that your `Dockerfile` uses the `http_proxy` and `https_proxy` environment variables during the build process.

## Contributing

This project is licensed under the [Apache 2.0 License](LICENSE).

We welcome contributions! Please see [docs/contributing.md](docs/contributing.md) for more information.

We follow [Google's Open Source Community Guidelines](https://opensource.google.com/conduct/).

## Disclaimer

This is not an officially supported Google product.

This project is not eligible for the Google Open Source Software Vulnerability Rewards Program.
