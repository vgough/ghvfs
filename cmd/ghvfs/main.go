package main

import (
	"os"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/vgough/ghvfs"
)

const (
	appVersion = "0.1"
)

var (
	app = kingpin.New("ghvfs", "GitHub Virtual FileSystem").
		Version(appVersion).
		Author("Valient Gough <vgough@pobox.com>").
		DefaultEnvars()

	debug      = app.Flag("debug", "Enable debug mode.").Bool()
	mountPoint = app.Arg("mountpoint", "Where to mount filesystem.").Required().ExistingDir()
	token      = app.Flag("token", "GitHub Auth Token.").Envar("GITHUB_TOKEN").String()
	github     = app.Flag("github", "GitHub URL, for private GHE.").String()
	cacheSize  = app.Flag("cache", "Size of attribute cache.").Int()
)

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	if *debug {
		ghvfs.Debug.SetOutput(os.Stderr)
	}

	opts := []ghvfs.Opt{}
	if *token != "" {
		opts = append(opts, ghvfs.WithToken(*token))
	}
	if *github != "" {
		opts = append(opts, ghvfs.WithGHEndpoint(*github))
	}
	if *cacheSize > 0 {
		opts = append(opts, ghvfs.WithCacheSize(*cacheSize))
	}

	fs := ghvfs.NewFS(opts...)
	pathFs := pathfs.NewPathNodeFs(fs, nil)
	fsOpts := nodefs.NewOptions()
	fsOpts.AttrTimeout = 60 * time.Second
	fsOpts.EntryTimeout = 60 * time.Second
	conn := nodefs.NewFileSystemConnector(pathFs.Root(), fsOpts)
	server, err := fuse.NewServer(conn.RawFS(), *mountPoint, &fuse.MountOptions{
		Name:           "githubfs",
		Debug:          *debug,
		RememberInodes: true,
	})
	if err != nil {
		ghvfs.Error.Printf("Mount fail: %v\n", err)
		os.Exit(1)
	}
	ghvfs.Debug.Println("Mounted!")
	server.Serve()
}
