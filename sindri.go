package sindri

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/frantjc/go-fn"
	"github.com/frantjc/sindri/steamcmd"
	"github.com/frantjc/sindri/thunderstore"
	xcontainerregistry "github.com/frantjc/sindri/x/containerregistry"
	xtar "github.com/frantjc/sindri/x/tar"
	xzip "github.com/frantjc/sindri/x/zip"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// ModMetadata stores metadata about an added mod.
type ModMetadata struct {
	LayerDigest string `json:"layerDigest,omitempty"`
	Version     string `json:"version,omitempty"`
}

// Metadata stores metadata about a downloaded game
// and added mods.
type Metadata struct {
	BaseLayerDigest string                 `json:"baseLayerDigest,omitempty"`
	Mods            map[string]ModMetadata `json:"mods,omitempty"`
}

// Sindri manages the files of a game and its mods.
type Sindri struct {
	SteamAppID         string
	BepInEx            *thunderstore.Package
	ThunderstoreClient *thunderstore.Client

	mu                 *sync.Mutex
	stateDir, rootDir  string
	img                v1.Image
	tag                *name.Tag
	metadata           *Metadata
	initialized        bool
	beta, betaPassword string
}

// Opt is an option to pass when creating
// a new Sindri instance.
type Opt func(*Sindri)

// WithRootDir sets a *Sindri's root directory
// where it will store any persistent data.
func WithRootDir(dir string) Opt {
	return func(s *Sindri) {
		s.rootDir = dir
	}
}

// WithStateDir sets a *Sindri's state directory
// where it will store any ephemeral data.
func WithStateDir(dir string) Opt {
	return func(s *Sindri) {
		s.stateDir = dir
	}
}

// WithBeta makes Sindri use the given Steam beta.
func WithBeta(beta string, password string) Opt {
	return func(s *Sindri) {
		s.beta = beta
		s.betaPassword = password
	}
}

const (
	// ImageRef is the image reference that Sindri
	// stores a game and its mods' files at inside
	// of it's .tar file.
	ImageRef = "frantj.cc/sindri"
)

// New creates a new Sindri instance with the given
// required arguments and options. Sindri can also be
// safely created directly so long as the exported
// fields are set to non-nil values.
func New(steamAppID string, bepInEx *thunderstore.Package, thunderstoreClient *thunderstore.Client, opts ...Opt) (*Sindri, error) {
	s := &Sindri{
		SteamAppID:         steamAppID,
		BepInEx:            bepInEx,
		ThunderstoreClient: thunderstoreClient,
	}

	return s, s.init(opts...)
}

// Mods returns the installed thunderstore.io packages.
func (s *Sindri) Mods() ([]thunderstore.Package, error) {
	pkgs := []thunderstore.Package{}

	for k, v := range s.metadata.Mods {
		pkg, err := thunderstore.ParsePackage(k + "-" + v.Version)
		if err != nil {
			return nil, err
		}

		pkgs = append(pkgs, *pkg)
	}

	return pkgs, nil
}

// AppUpdate uses `steamcmd` to installed or update
// the game that Sindri is managing.
func (s *Sindri) AppUpdate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.init(); err != nil {
		return err
	}

	tmp, err := os.MkdirTemp(s.stateDir, "base-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	cmd, err := steamcmd.Run(ctx, &steamcmd.Commands{
		ForceInstallDir: tmp,
		AppUpdate:       s.SteamAppID,
		Beta:            s.beta,
		BetaPassword:    s.betaPassword,
		Validate:        true,
	})
	if err != nil {
		return err
	}

	if err = cmd.Run(); err != nil {
		return err
	}

	layer, err := xcontainerregistry.LayerFromDir(tmp)
	if err != nil {
		return err
	}

	digest, err := layer.Digest()
	if err != nil {
		return err
	}

	if s.metadata.BaseLayerDigest == digest.String() {
		return nil
	}

	layers, err := s.modLayers()
	if err != nil {
		return err
	}

	layers = append(layers, layer)

	if s.img, err = mutate.AppendLayers(empty.Image, layers...); err != nil {
		return err
	}

	s.metadata.BaseLayerDigest = digest.String()

	return s.save()
}

// AddMods installs or updates the given mods and their
// dependencies using thunderstore.io.
func (s *Sindri) AddMods(ctx context.Context, mods ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.init(); err != nil {
		return err
	}

	layers, err := s.img.Layers()
	if err != nil {
		return err
	}

	for _, mod := range append(mods, s.BepInEx.Versionless().String()) {
		pkg, err := thunderstore.ParsePackage(mod)
		if err != nil {
			return err
		}

		var (
			modKey      = pkg.Versionless().String()
			modMeta, ok = s.metadata.Mods[modKey]
		)
		if ok {
			if modMeta.Version == pkg.Version {
				continue
			}
		}

		tmpDir, err := os.MkdirTemp(s.stateDir, pkg.Fullname()+"-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		if err := s.extractModsAndDependenciesToDir(ctx, tmpDir, mod); err != nil {
			return err
		}

		modLayer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
			return xtar.Compress(tmpDir), nil
		})
		if err != nil {
			return err
		}

		digest, err := modLayer.Digest()
		if err != nil {
			return err
		}

		if ok && modMeta.LayerDigest == digest.String() {
			continue
		}

		fileteredLayers := []v1.Layer{}

		for _, layer := range layers {
			digest, err := layer.Digest()
			if err != nil {
				return err
			}

			if digest.String() != modMeta.LayerDigest {
				fileteredLayers = append(fileteredLayers, layer)
			}
		}

		layers = fileteredLayers
		layers = append(layers, modLayer)

		s.metadata.Mods[modKey] = ModMetadata{
			Version:     pkg.Version,
			LayerDigest: digest.String(),
		}
	}

	if s.img, err = mutate.AppendLayers(s.img, layers...); err != nil {
		return err
	}

	return s.save()
}

// RemoveMods removes the given mods.
func (s *Sindri) RemoveMods(_ context.Context, mods ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.init(); err != nil {
		return err
	}

	layers, err := s.img.Layers()
	if err != nil {
		return err
	}

	for _, mod := range mods {
		pkg, err := thunderstore.ParsePackage(mod)
		if err != nil {
			return err
		}

		if pkg.Versionless().String() == s.BepInEx.Versionless().String() {
			return fmt.Errorf("cannot remove BepInEx")
		}

		var (
			modKey      = pkg.Versionless().String()
			modMeta, ok = s.metadata.Mods[modKey]
		)
		if !ok {
			continue
		}

		fileteredLayers := []v1.Layer{}

		for _, layer := range layers {
			digest, err := layer.Digest()
			if err != nil {
				return err
			}

			if digest.String() != modMeta.LayerDigest {
				fileteredLayers = append(fileteredLayers, layer)
			}
		}

		layers = fileteredLayers
	}

	if s.img, err = mutate.AppendLayers(empty.Image, layers...); err != nil {
		return err
	}

	return s.save()
}

// Extract returns an io.ReadCloser containing a tarball
// containing the files of the game and its mods.
func (s *Sindri) Extract() (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return mutate.Extract(s.img), nil
}

// ExtractMods returns an io.ReadCloser containing a tarball
// containing the files just the game's mods.
func (s *Sindri) ExtractMods() (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	layers, err := s.modLayers()
	if err != nil {
		return nil, err
	}

	img, err := mutate.AppendLayers(empty.Image, layers...)
	if err != nil {
		return nil, err
	}

	return mutate.Extract(img), nil
}

func (s *Sindri) save() error {
	var (
		tmpTarPath = filepath.Join(s.rootDir, "sindri.tmp.tar")
		tmpDbPath  = filepath.Join(s.rootDir, "sindri.tmp.json")
	)

	if err := tarball.WriteToFile(tmpTarPath, name.MustParseReference(ImageRef), s.img); err != nil {
		return err
	}

	if err := os.Rename(tmpTarPath, s.tarPath()); err != nil {
		return err
	}

	img, err := tarball.ImageFromPath(s.tarPath(), s.tag)
	if err != nil {
		return err
	}

	s.img = img

	db, err := os.Create(tmpDbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err = json.NewEncoder(db).Encode(s.metadata); err != nil {
		return err
	}

	if err := os.Rename(tmpDbPath, s.dbPath()); err != nil {
		return err
	}

	return nil
}

func (s *Sindri) extractModsAndDependenciesToDir(ctx context.Context, dir string, mods ...string) error {
	for _, mod := range mods {
		pkg, err := thunderstore.ParsePackage(mod)
		if err != nil {
			return err
		}

		var (
			modKey      = pkg.Versionless().String()
			bepInExKey  = s.BepInEx.Versionless().String()
			isBepInEx   = modKey == bepInExKey
			modMeta, ok = s.metadata.Mods[modKey]
		)
		if ok {
			if modMeta.Version == pkg.Version {
				continue
			}
		}

		fmt.Println(modKey+" == ", bepInExKey+" ?", isBepInEx)

		// The pkg doesn't need a version to get the metadata
		// or the archive, but we want the version so we know
		// what version is installed, so we make sure that we
		// have it. We also need to know its dependencies.
		pkgMeta, err := s.ThunderstoreClient.GetPackageMetadata(ctx, pkg)
		if err != nil {
			return err
		}

		var (
			dependencies = fn.Filter(pkgMeta.Dependencies, func(dependency string, _ int) bool {
				return !strings.HasPrefix(dependency, bepInExKey)
			})
		)
		if err := s.extractModsAndDependenciesToDir(ctx, dir, dependencies...); err != nil {
			return err
		}

		if pkg.Version == "" && pkgMeta.Latest != nil {
			pkg = &pkgMeta.Latest.Package
		}

		pkgZip, err := s.ThunderstoreClient.GetPackageZip(ctx, pkg)
		if err != nil {
			return err
		}
		defer pkgZip.Close()

		pkgZipRdr, err := zip.NewReader(pkgZip, pkgZip.Size())
		if err != nil {
			return err
		}

		pkgZipRdr.File = fn.Reduce(pkgZipRdr.File, func(acc []*zip.File, cur *zip.File, _ int) []*zip.File {
			norm := strings.ReplaceAll(cur.Name, "\\", "/")

			if isBepInEx {
				name, err := filepath.Rel(s.BepInEx.Name, norm)
				if err != nil {
					return acc
				}

				if strings.Contains(name, "..") {
					return acc
				}

				cur.Name = name
			} else {
				cur.Name = filepath.Join("BepInEx/plugins", pkg.Fullname(), norm)
			}

			return append(acc, cur)
		}, []*zip.File{})

		if err := xzip.Extract(pkgZipRdr, dir); err != nil {
			return err
		}
	}

	return nil
}

func (s *Sindri) dbPath() string {
	return filepath.Join(s.rootDir, "sindri.json")
}

func (s *Sindri) tarPath() string {
	return filepath.Join(s.rootDir, "sindri.tar")
}

func (s *Sindri) modLayers() ([]v1.Layer, error) {
	layers, err := s.img.Layers()
	if err != nil {
		return nil, err
	}

	filteredLayers := []v1.Layer{}

	for _, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return nil, err
		}

		if digest.String() != s.metadata.BaseLayerDigest {
			filteredLayers = append(filteredLayers, layer)
		}
	}

	return filteredLayers, nil
}

func (s *Sindri) init(opts ...Opt) error {
	switch {
	case s.SteamAppID == "":
		return fmt.Errorf("empty SteamAppID")
	case s.BepInEx == nil:
		return fmt.Errorf("nil BepInEx Package")
	case s.ThunderstoreClient == nil:
		return fmt.Errorf("nil ThunderstoreClient")
	}

	if s.initialized {
		return nil
	}

	s.img = empty.Image
	s.mu = new(sync.Mutex)
	s.metadata = &Metadata{
		Mods: map[string]ModMetadata{},
	}

	for _, opt := range opts {
		opt(s)
	}

	tag, err := name.NewTag(ImageRef)
	if err != nil {
		return err
	}
	s.tag = &tag

	if s.rootDir == "" || s.stateDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		if s.rootDir == "" {
			s.rootDir = filepath.Join(wd, "root")
		}

		if s.stateDir == "" {
			s.stateDir = filepath.Join(wd, "state")
		}
	}

	if err := os.MkdirAll(s.stateDir, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(s.rootDir, 0755); err != nil {
		return err
	}

	if fi, err := os.Stat(s.tarPath()); err == nil && !fi.IsDir() && fi.Size() > 0 {
		if s.img, err = tarball.ImageFromPath(s.tarPath(), s.tag); err != nil {
			return err
		}
	}

	if fi, err := os.Stat(s.dbPath()); err == nil && !fi.IsDir() && fi.Size() > 0 {
		db, err := os.Open(s.dbPath())
		if err != nil {
			return err
		}
		defer db.Close()

		if err = json.NewDecoder(db).Decode(s.metadata); err != nil {
			return err
		}
	}

	s.initialized = true

	return nil
}
