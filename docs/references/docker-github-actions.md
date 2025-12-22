# Docker Build GitHub Actions

Source: https://docs.docker.com/build/ci/github-actions/

GitHub Actions is a popular CI/CD platform for automating your build, test, and
deployment pipeline. Docker provides a set of official GitHub Actions for you to
use in your workflows. These official actions are reusable, easy-to-use
components for building, annotating, and pushing images.

## Available GitHub Actions

- [Build and push Docker images](https://github.com/marketplace/actions/build-and-push-docker-images): build and push Docker images with BuildKit.
- [Docker Buildx Bake](https://github.com/marketplace/actions/docker-buildx-bake): enables using high-level builds with Bake.
- [Docker Login](https://github.com/marketplace/actions/docker-login): sign in to a Docker registry.
- [Docker Setup Buildx](https://github.com/marketplace/actions/docker-setup-buildx): creates and boots a BuildKit builder.
- [Docker Metadata action](https://github.com/marketplace/actions/docker-metadata-action): extracts metadata from Git reference and GitHub events to generate tags, labels, and annotations.
- [Docker Setup Compose](https://github.com/marketplace/actions/docker-setup-compose): installs and sets up Compose.
- [Docker Setup Docker](https://github.com/marketplace/actions/docker-setup-docker): installs Docker Engine.
- [Docker Setup QEMU](https://github.com/marketplace/actions/docker-setup-qemu): installs QEMU static binaries for multi-platform builds.
- [Docker Scout](https://github.com/docker/scout-action): analyze Docker images for security vulnerabilities.

## Examples

- [Add image annotations with GitHub Actions](https://docs.docker.com/build/ci/github-actions/annotations/)
- [Add SBOM and provenance attestations with GitHub Actions](https://docs.docker.com/build/ci/github-actions/attestations/)
- [Validating build configuration with GitHub Actions](https://docs.docker.com/build/ci/github-actions/checks/)
- [Using secrets with GitHub Actions](https://docs.docker.com/build/ci/github-actions/secrets/)
- [GitHub Actions build summary](https://docs.docker.com/build/ci/github-actions/build-summary/)
- [Configuring your GitHub Actions builder](https://docs.docker.com/build/ci/github-actions/configure-builder/)
- [Cache management with GitHub Actions](https://docs.docker.com/build/ci/github-actions/cache/)
- [Copy image between registries with GitHub Actions](https://docs.docker.com/build/ci/github-actions/copy-image-registries/)
- [Export to Docker with GitHub Actions](https://docs.docker.com/build/ci/github-actions/export-docker/)
- [Local registry with GitHub Actions](https://docs.docker.com/build/ci/github-actions/local-registry/)
- [Multi-platform image with GitHub Actions](https://docs.docker.com/build/ci/github-actions/multi-platform/)
- [Named contexts with GitHub Actions](https://docs.docker.com/build/ci/github-actions/named-contexts/)
- [Push to multiple registries with GitHub Actions](https://docs.docker.com/build/ci/github-actions/push-multi-registries/)
- [Reproducible builds with GitHub Actions](https://docs.docker.com/build/ci/github-actions/reproducible-builds/)
- [Share built image between jobs with GitHub Actions](https://docs.docker.com/build/ci/github-actions/share-image-jobs/)
- [Manage tags and labels with GitHub Actions](https://docs.docker.com/build/ci/github-actions/manage-tags-labels/)
- [Test before push with GitHub Actions](https://docs.docker.com/build/ci/github-actions/test-before-push/)
- [Update Docker Hub description with GitHub Actions](https://docs.docker.com/build/ci/github-actions/update-dockerhub-desc/)

---

# Multi-platform image with GitHub Actions

Source: https://docs.docker.com/build/ci/github-actions/multi-platform/

You can build multi-platform images using the `platforms` option.

> **Note:**
> - For a list of available platforms, see the [Docker Setup Buildx](https://github.com/marketplace/actions/docker-setup-buildx) action.
> - If you want support for more platforms, you can use QEMU with the [Docker Setup QEMU](https://github.com/docker/setup-qemu-action) action.

## Basic Multi-platform Build

```yaml
name: ci

on:
  push:

jobs:
  docker:
    runs-on: ubuntu-latest
    steps:
      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ vars.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          platforms: linux/amd64,linux/arm64
          push: true
          tags: user/app:latest
```

## Build and load multi-platform images

The default Docker setup for GitHub Actions runners does not support loading
multi-platform images to the local image store of the runner after building
them. To load a multi-platform image, you need to enable the containerd image
store option for the Docker Engine.

```yaml
name: ci

on:
  push:

jobs:
  docker:
    runs-on: ubuntu-latest
    steps:
      - name: Set up Docker
        uses: docker/setup-docker-action@v4
        with:
          daemon-config: |
            {
              "debug": true,
              "features": {
                "containerd-snapshotter": true
              }
            }

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ vars.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          platforms: linux/amd64,linux/arm64
          load: true
          tags: user/app:latest
```

## Distribute build across multiple runners

Building multiple platforms on the same runner can significantly extend build
times. By distributing platform-specific builds across multiple runners using a
matrix strategy, you can drastically reduce build durations.

```yaml
name: ci

on:
  push:

env:
  REGISTRY_IMAGE: user/app

jobs:
  build:
    strategy:
      fail-fast: false
      matrix:
        include:
        - platform: linux/amd64
          runner: ubuntu-latest
        - platform: linux/arm64
          runner: ubuntu-24.04-arm
    runs-on: ${{ matrix.runner }}
    steps:
      - name: Prepare
        run: |
          platform=${{ matrix.platform }}
          echo "PLATFORM_PAIR=${platform//\//-}" >> $GITHUB_ENV

      - name: Docker meta
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY_IMAGE }}

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ vars.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Build and push by digest
        id: build
        uses: docker/build-push-action@v6
        with:
          platforms: ${{ matrix.platform }}
          labels: ${{ steps.meta.outputs.labels }}
          tags: ${{ env.REGISTRY_IMAGE }}
          outputs: type=image,push-by-digest=true,name-canonical=true,push=true

      - name: Export digest
        run: |
          mkdir -p ${{ runner.temp }}/digests
          digest="${{ steps.build.outputs.digest }}"
          touch "${{ runner.temp }}/digests/${digest#sha256:}"

      - name: Upload digest
        uses: actions/upload-artifact@v4
        with:
          name: digests-${{ env.PLATFORM_PAIR }}
          path: ${{ runner.temp }}/digests/*
          if-no-files-found: error
          retention-days: 1

  merge:
    runs-on: ubuntu-latest
    needs:
      - build
    steps:
      - name: Download digests
        uses: actions/download-artifact@v4
        with:
          path: ${{ runner.temp }}/digests
          pattern: digests-*
          merge-multiple: true

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ vars.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Docker meta
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY_IMAGE }}
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}

      - name: Create manifest list and push
        working-directory: ${{ runner.temp }}/digests
        run: |
          docker buildx imagetools create $(jq -cr '.tags | map("-t " + .) | join(" ")' <<< "$DOCKER_METADATA_OUTPUT_JSON") \
            $(printf '${{ env.REGISTRY_IMAGE }}@sha256:%s ' *)

      - name: Inspect image
        run: |
          docker buildx imagetools inspect ${{ env.REGISTRY_IMAGE }}:${{ steps.meta.outputs.version }}
```

## Key Notes

1. **QEMU is required** for cross-platform builds (e.g., building arm64 on amd64 runner)
2. **Buildx is required** for multi-platform builds
3. **`docker/build-push-action@v6`** is the latest version (as of 2025)
4. For faster builds, use **native ARM runners** (`ubuntu-24.04-arm`) instead of QEMU emulation
5. **Matrix strategy** allows building each platform on a dedicated runner
