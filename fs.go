package ghvfs

import (
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

// Opt is an option for filesystem construction.
type Opt func(*config)

// WithGHEndpoint allows specifying an optional GitHub base URL.
// This can be used to point to an internal GitHub Enterprise endpoint.
func WithGHEndpoint(base string) Opt {
	return func(cfg *config) {
		v3URL := base + "/api/v3/"
		u, err := url.Parse(v3URL)
		if err != nil {
			log.Fatal("unable to parse github url")
		}
		cfg.v3 = u
	}
}

// WithToken provides a GitHub token for authentication.
// The token must provide read access to contents for the orgs and repos being
// accessed.
func WithToken(t string) Opt {
	return func(cfg *config) {
		cfg.token = t
	}
}

// WithCacheSize provides a cache size for internal attribute caching.
func WithCacheSize(size int) Opt {
	return func(cfg *config) {
		cfg.cacheSize = size
	}
}

type config struct {
	v3        *url.URL
	token     string
	cacheSize int
}

type gfs struct {
	pathfs.FileSystem

	client *github.Client

	// cache maps full path to *stat.
	// Since contents are tied to a specific ref, they are immutable and can
	// be cached indefinitely to speed up access.
	// This can cache both positive and negative results.
	cache *lru.Cache
}

type stat struct {
	attr    *fuse.Attr
	status  fuse.Status
	entries []fuse.DirEntry // for directories
}

// NewFS creates a new filesystem.
func NewFS(opts ...Opt) pathfs.FileSystem {
	cfg := config{
		cacheSize: 4096,
	}
	for _, o := range opts {
		o(&cfg)
	}

	cache, err := lru.New(cfg.cacheSize)
	if err != nil {
		log.Fatal("unable to construct cache:", err)
	}
	fs := &gfs{
		FileSystem: pathfs.NewDefaultFileSystem(),
		cache:      cache,
	}

	// Auth config.
	var ts oauth2.TokenSource
	if cfg.token != "" {
		ts = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: cfg.token},
		)
	}
	tc := oauth2.NewClient(context.Background(), ts)

	// Initialize V3 API client.
	fs.client = github.NewClient(tc)
	if cfg.v3 != nil {
		Debug.Printf("connecting to %v", cfg.v3)
		fs.client.BaseURL = cfg.v3
		fs.client.UploadURL = nil
	}

	return fs
}

func (fs *gfs) StatFs(name string) *fuse.StatfsOut {
	return &fuse.StatfsOut{}
}

func (fs *gfs) GetAttr(name string, fctx *fuse.Context) (*fuse.Attr, fuse.Status) {
	Debug.Printf("getattr %q", name)
	if cached, ok := fs.cache.Get(name); ok {
		c := cached.(*stat)
		return c.attr, c.status
	}

	parts := strings.SplitN(name, string(os.PathSeparator), 4)
	if len(parts) <= 3 {
		dirAttr := newDirInfo(0111)
		return &dirAttr, fuse.OK
	}

	attr, status := fs.getContent(parts[0], parts[1], parts[2], parts[3], fctx)
	fs.cache.Add(name, &stat{
		attr:   attr,
		status: status,
	})
	return attr, status
}

func (fs *gfs) OpenDir(name string, fctx *fuse.Context) (entries []fuse.DirEntry, status fuse.Status) {
	Debug.Printf("opendir %q", name)
	if cached, ok := fs.cache.Get(name); ok {
		c := cached.(*stat)
		if c.entries != nil {
			return c.entries, c.status
		}
	}

	parts := strings.SplitN(name, string(os.PathSeparator), 4)
	if len(parts) < 3 {
		return nil, fuse.EACCES
	}

	ctx := context.Background()
	org := parts[0]
	repo := parts[1]
	var path string
	if len(parts) == 4 {
		path = parts[3]
	}
	opt := github.RepositoryContentGetOptions{
		Ref: parts[2],
	}
	// TODO: iterate on too many results?
	_, ds, _, err := fs.client.Repositories.GetContents(ctx, org, repo, path, &opt)
	if err != nil {
		// TODO: distinguish error values
		return nil, fuse.ENOENT
	}

	if ds == nil {
		return nil, fuse.ENOTDIR
	}

	for _, ent := range ds {
		entries = append(entries, fuse.DirEntry{
			Name: ent.GetName(),
			Mode: 0444,
		})
		var attr fuse.Attr
		if ent.GetType() == "file" {
			attr = newFileInfo()
		} else {
			attr = newDirInfo(0555)
		}
		attr.Size = uint64(ent.GetSize())
		fs.cache.Add(filepath.Join(name, ent.GetName()), &stat{
			attr:   &attr,
			status: fuse.OK,
		})
	}

	attr := newDirInfo(0555)
	fs.cache.Add(name, &stat{
		attr:    &attr,
		status:  status,
		entries: entries,
	})
	return entries, fuse.OK
}

func (fs *gfs) getContent(org, repo, id, path string, fctx *fuse.Context) (*fuse.Attr, fuse.Status) {
	Debug.Printf("get content %q / %q / %q/ %q", org, repo, id, path)
	ctx := context.Background()
	opt := github.RepositoryContentGetOptions{
		Ref: id,
	}
	fc, ds, _, err := fs.client.Repositories.GetContents(ctx, org, repo, path, &opt)
	if err != nil {
		Debug.Printf("err: %v", err)
		// TODO: distinguish different error types
		return nil, fuse.ENOENT
	}

	var attr fuse.Attr
	if ds == nil {
		attr = newFileInfo()
		attr.Size = uint64(fc.GetSize())
	} else {
		attr = newDirInfo(0555)
	}
	return &attr, fuse.OK
}

func newDirInfo(mask uint32) fuse.Attr {
	var info fuse.Attr
	now := time.Now()
	info.SetTimes(&now, &now, &now)
	info.Mode = fuse.S_IFDIR | mask
	return info
}

func newFileInfo() fuse.Attr {
	var info fuse.Attr
	now := time.Now()
	info.SetTimes(&now, &now, &now)
	info.Mode = fuse.S_IFREG | 0444
	return info
}

func (fs *gfs) Open(name string, flags uint32, fctx *fuse.Context) (nodefs.File, fuse.Status) {
	parts := strings.SplitN(name, string(os.PathSeparator), 4)
	if len(parts) != 4 {
		return nil, fuse.ENOENT
	}

	return newContentNode(fs, flags, parts[0], parts[1], parts[2], parts[3])
}
