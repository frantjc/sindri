package dagger

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"

	"dagger.io/dagger"
	"github.com/Khan/genqlient/graphql"
	"github.com/frantjc/sindri"
	"github.com/frantjc/sindri/internal/httputil"
)

type ImageBuilder struct {
	WorkDir          string
	NameToModule     map[string]string
	ModulesDirectory string
	ModulesRef       string
	ModulesURL       string
}

var (
	defaultModulesURL   = "https://github.com/frantjc/sindri"
	defaultNameToModule = map[string]string{
		"abioticfactor": "abioticfactor",
		"2857200":       "abioticfactor",
		"astroneer":     "astroneer",
		"corekeeper":    "corekeeper",
		"1963720":       "corekeeper",
		"enshrouded":    "enshrouded",
		"palworld":      "palworld",
		"2394010":       "palworld",
		"valheim":       "valheim",
		"896660":        "valheim",
		"satisfactory":  "satisfactory",
		"1690800":       "satisfactory",
	}
)

func (i *ImageBuilder) BuildImage(ctx context.Context, name, branch string) (sindri.Opener, error) {
	if i == nil {
		i = &ImageBuilder{}
	}

	if len(i.NameToModule) == 0 {
		i.NameToModule = defaultNameToModule
	}

	if i.ModulesURL == "" {
		i.ModulesURL = defaultModulesURL
	}

	if module, ok := i.NameToModule[name]; ok {
		if i.WorkDir == "" {
			i.WorkDir = os.TempDir()
		}

		if err := os.MkdirAll(i.WorkDir, 0700); err != nil {
			return nil, err
		}

		tarball := path.Join(i.WorkDir, fmt.Sprintf("%s-%s.tar", module, branch))

		log, err := os.Create(path.Join(i.WorkDir, fmt.Sprintf("%s-%s.log", module, branch)))
		if err != nil {
			return nil, err
		}
		defer log.Close()

		dag, err := dagger.Connect(ctx, dagger.WithLogOutput(log))
		if err != nil {
			return nil, err
		}
		defer dag.Close()

		modulesGit := dag.Git(i.ModulesURL)
		modulesDirectory := modulesGit.Head().Tree()

		if i.ModulesDirectory != "" {
			modulesDirectory = dag.Host().Directory(i.ModulesDirectory)
		} else if i.ModulesRef != "" {
			modulesDirectory = modulesGit.Ref(i.ModulesRef).Tree()
		}

		if err := modulesDirectory.AsModule(dagger.DirectoryAsModuleOpts{SourceRootPath: module}).Serve(ctx); err != nil {
			return nil, err
		}

		if err := dag.GraphQLClient().
			MakeRequest(ctx,
				&graphql.Request{
					Query: fmt.Sprintf(`query{%s{container(branch:"%s"){asTarball{export(path:"%s")}}}}`, module, branch, tarball),
				},
				&graphql.Response{},
			); err != nil {
			return nil, err
		}

		return sindri.FileOpener(tarball), nil
	}

	return nil, httputil.NewHTTPStatusCodeError(
		fmt.Errorf("unknown image name %s", name),
		http.StatusNotFound,
	)
}
