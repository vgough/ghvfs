# ghvfs
Virtual filesystem for GitHub content

## Usage

First, mount the filesystem somewhere:
```bash
govfs /tmp/gh
```

Paths in the virtual filesystem are in the form `/[org]/[repo]/[ref]/[path]`.

For example, to see files as they were at the time of the first commit to `ghvfs`:

```bash
$ ls /tmp/gh/vgough/ghvfs/cc289b54162f8e9521042e0efb79c779fc89cad9/
cmd  files.go  fs.go  Gopkg.lock  Gopkg.toml  LICENSE  logs.go  README.md  vendor
```

## GitHub Enterprise

To use with an enterprise installation, pass a GitHub URL and token.  These can
be passed using command line flags and/or environment variables.  For example:

```bash
GITHUB_TOKEN=xxx ghvfs --github=https://github.internal.example.com/ /tmp/gh
```

## Performance

ghvfs is intended to be used when *sparse* access is needed to a github repository.
For example, when running a tool which might pull in other references.  Instead
of cloning the entire repo locally, this allows files to be pulled on-demand.

If you intend to a large fraction of the files in the repository, it is almost
certainly faster to clone the repo (with limited depth) and use the local copy.

ghvfs by default keeps an LRU cache of the last few thousand file entries and
directory listings, to avoid repeating GitHub queries.  This can be controlled
by the `--cache` flag.