package resource

import (
	"os"
	"path/filepath"

	"github.com/gbl08ma/sqalx"
	"github.com/yarf-framework/yarf"
)

// AndroidClientConfig composites resource
type AndroidClientConfig struct {
	resource
}

// apiAndroidClientConfig contains configuration information for the Android client
type apiAndroidClientConfig struct {
	HelpOverlays map[string]string `msgpack:"helpOverlays" json:"helpOverlays"`
	Infra        map[string]string `msgpack:"infra" json:"infra"`
}

// WithNode associates a sqalx Node with this resource
func (r *AndroidClientConfig) WithNode(node sqalx.Node) *AndroidClientConfig {
	r.node = node
	return r
}

// Get serves HTTP GET requests on this resource
func (r *AndroidClientConfig) Get(c *yarf.Context) error {
	resp := apiAndroidClientConfig{
		HelpOverlays: make(map[string]string),
		Infra:        make(map[string]string),
	}

	buildPaths := func(root string, m map[string]string) error {
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			pathOnApp, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			m[filepath.ToSlash(pathOnApp)] = filepath.ToSlash(path)
			return nil
		})
	}

	err := buildPaths("stationkb/overlayfs/help", resp.HelpOverlays)
	if err != nil {
		return err
	}

	err = buildPaths("stationkb/overlayfs/infra", resp.Infra)
	if err != nil {
		return err
	}
	RenderData(c, resp, "s-maxage=10")
	return nil
}
