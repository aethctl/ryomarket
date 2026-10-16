# Catalog model

Each application exists once in `internal/catalog/seed.json` and has per-distro provider variants.

```json
{
  "id": "org.obsproject.Studio",
  "name": "OBS Studio",
  "variants": {
    "fedora": [
      { "kind": "dnf", "package": "obs-studio", "recommended": true },
      { "kind": "flatpak", "package": "com.obsproject.Studio" }
    ]
  }
}
```

Stable Ryomarket IDs are application identities, not package names. This is what keeps native, Flatpak, Nix and portable builds on a single product page.

Provider metadata should declare `recommended`, `official`, `sandboxed` and `verified` only when the claim is known. `verified` means the Ryomarket install recipe is curated and reversible. It is not a security endorsement of the upstream application.

Portable recipes may use a direct HTTPS URL or a GitHub release repository plus an asset pattern. Generic arbitrary archives are intentionally unsupported.
