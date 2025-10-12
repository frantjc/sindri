package e2e_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
)

var (
	registry = os.Getenv("SINDRI_TEST_REGISTRY")
)

func TestPull(t *testing.T) {
	if !assert.NotEmpty(t, registry) {
		assert.FailNow(t, "SINDRI_TEST_REGISTRY must be set")
	}

	updates := make(chan v1.Update, 121)
	defer close(updates)

	go func() {
		for update := range updates {
			if assert.NoError(t, update.Error) {
				t.Logf("%d/%d", update.Complete, update.Total)
			}
		}
	}()

	for _, steamapp := range []string{"valheim", "corekeeper"} {
		ref, err := name.ParseReference(fmt.Sprintf("%s/%s", registry, steamapp))
		if !assert.NoError(t, err) {
			continue
		}

		t.Logf("pulling %s", ref)

		img, err := remote.Image(
			ref,
			remote.WithContext(t.Context()),
			remote.WithProgress(updates),
			// TODO(frantjc): Why is this needed?
			remote.WithTransport(&http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}),
		)
		if assert.NoError(t, err) {
			assert.NotNil(t, img)
		}

		t.Logf("pulled %s", ref)
	}
}
