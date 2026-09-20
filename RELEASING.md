# Releasing

A release is a tag. Everything else is the workflow.

## Once, before the first release

1. Create a fine-grained personal access token with **Contents: read and
   write** on `avison9/homebrew-tap` only, and nothing else.
2. Store it on this repository as the secret `HOMEBREW_TAP_GITHUB_TOKEN`
   (Settings, Secrets and variables, Actions). The release workflow's own
   `GITHUB_TOKEN` cannot push to another repository, which is why the tap
   needs its own.

Without the secret the release still builds and publishes on GitHub; only
the cask push fails, and the run says so.

## Every release

```
git checkout main && git pull --ff-only
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

The `release` workflow then:

1. runs the tests,
2. builds `cdclint` for linux, darwin and windows on amd64 and arm64, with
   `main.version` set to the tag and reproducible flags,
3. writes one archive per platform plus `checksums.txt`,
4. signs `checksums.txt` with cosign, keyless, through GitHub's OIDC
   identity, so anyone can verify a download came from this workflow,
5. creates the GitHub Release with a changelog from the commits since the
   previous tag,
6. writes `Casks/cdclint.rb` to `avison9/homebrew-tap`,
7. moves the major tag (`v0`, later `v1`) to this release, which is what
   `uses: avison9/cdclint@v1` resolves.

Check the run, then check the three places a user meets it: the Release
page, `brew install avison9/tap/cdclint` on a Mac or Linux box, and a
workflow using the action.

## Trying the build without releasing

```
docker run --rm -v "$PWD":/src -w /src -e HOMEBREW_TAP_GITHUB_TOKEN=x \
  --entrypoint sh goreleaser/goreleaser:latest -c \
  "git config --global --add safe.directory /src && goreleaser release --snapshot --clean --skip=sign,publish"
```

`dist/` then holds every archive and the generated cask. It is written by
root inside the container; remove it the same way.

## Version numbers

`v0.x` until the ten rules in the README's table all say v0.1 and the
tool has run in someone else's CI for a month. Breaking a flag or a finding's
text after `v1.0.0` is a major.
