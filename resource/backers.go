package resource

import (
	"net/http"
	"os"

	"github.com/thoas/go-funk"
	"github.com/underlx/disturbancesmlx/utils"
	"github.com/yarf-framework/yarf"
)

// Backers composites resource
type Backers struct {
	resource
}

// Get serves HTTP GET requests on this resource
func (r *Backers) Get(c *yarf.Context) error {
	return r.headOrGet(c)
}

// Head serves HTTP HEAD requests on this resource
func (r *Backers) Head(c *yarf.Context) error {
	return r.headOrGet(c)
}

func (r *Backers) headOrGet(c *yarf.Context) error {
	locale := c.Request.URL.Query().Get("locale")
	if locale == "" {
		locale = "en"
	}
	if !funk.ContainsString(utils.SupportedLocales[:], locale) {
		return yarf.ErrorNotFound()
	}
	filename := "stationkb/" + locale + "/backers.html"

	info, err := os.Stat(filename)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	// does not send the body if this is a HEAD request
	http.ServeContent(c.Response, c.Request, "backers.html", info.ModTime(), file)

	return nil
}
