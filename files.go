package ghvfs

import (
	"github.com/google/go-github/github"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"golang.org/x/net/context"
)

type fnode struct {
	nodefs.File
	fs                   *gfs
	org, repo, ref, path string
	content              []byte
}

func newContentNode(fs *gfs, flags uint32, org, repo, ref, path string) (nodefs.File, fuse.Status) {
	return &fnode{
		File: nodefs.NewReadOnlyFile(nodefs.NewDefaultFile()),
		fs:   fs,
		org:  org,
		repo: repo,
		ref:  ref,
		path: path,
	}, fuse.OK
}

func (f *fnode) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	if f.content == nil {
		Debug.Printf("loading content for %q", f.path)
		ctx := context.Background()
		opt := github.RepositoryContentGetOptions{
			Ref: f.ref,
		}
		fc, _, _, err := f.fs.client.Repositories.GetContents(ctx, f.org, f.repo, f.path, &opt)
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		content, err := fc.GetContent()
		if err != nil {
			return nil, fuse.ToStatus(err)
		}
		f.content = []byte(content)
	}

	start := off
	end := off + int64(len(buf))
	if start > int64(len(f.content)) {
		start = 0
		end = 0
	}
	if end > int64(len(f.content)) {
		end = int64(len(f.content))
	}
	return fuse.ReadResultData(f.content[start:end]), fuse.OK
}
