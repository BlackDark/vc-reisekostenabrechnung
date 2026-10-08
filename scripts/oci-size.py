#!/usr/bin/env python3
"""Print compressed size per platform from a buildx OCI archive."""

import json
import sys
import tarfile


def main() -> None:
    path = sys.argv[1]
    with tarfile.open(path) as tar:
        index = json.load(tar.extractfile("index.json"))
        manifests = index.get("manifests", [])
        for desc in manifests:
            digest = desc["digest"].split(":", 1)[1]
            plat = desc.get("platform") or {}
            blob = json.load(tar.extractfile(f"blobs/sha256/{digest}"))
            if blob.get("mediaType", "").endswith("index") or "manifests" in blob and "layers" not in blob:
                for nested in blob.get("manifests", []):
                    nd = nested["digest"].split(":", 1)[1]
                    np = nested.get("platform") or plat
                    man = json.load(tar.extractfile(f"blobs/sha256/{nd}"))
                    print_size(np, man)
                continue
            print_size(plat, blob)


def print_size(plat: dict, man: dict) -> None:
    size = man.get("config", {}).get("size", 0)
    for layer in man.get("layers", []):
        size += layer.get("size", 0)
    arch = plat.get("architecture", "?")
    variant = plat.get("variant")
    name = f"linux/{arch}" + (f"/{variant}" if variant else "")
    print(f"image size {name}: {size} bytes ({size / 1024 / 1024:.1f} MiB compressed layers)")


if __name__ == "__main__":
    main()
