package widgets

import (
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/core"
)

// The images the app shows from the network and from files, decoded once
// and kept, as Image.network's and Image.file's cache does. As there, a
// failed load is not kept: an image shown again after pictureRetry loads
// anew.
type picture struct {
	bmp     *ui.Bitmap
	err     error
	failed  time.Time
	loading bool
}

const pictureRetry = 5 * time.Second

var (
	pictures   = map[string]*picture{}
	picturesMu sync.Mutex
	webClient  = &http.Client{Timeout: 30 * time.Second}
)

// Headers, when set, adds the daemon's token to its own URLs.
var Headers func(url string) map[string]string

func loadPicture(key string, fetch func() ([]byte, error)) *picture {
	picturesMu.Lock()
	defer picturesMu.Unlock()
	p := pictures[key]
	if p != nil && (p.err == nil || time.Since(p.failed) < pictureRetry) {
		return p
	}
	p = &picture{loading: true}
	pictures[key] = p
	go func() {
		data, err := fetch()
		var bmp *ui.Bitmap
		if err == nil {
			bmp, err = ui.DecodeBitmap(data)
		}
		core.Post(func() { p.bmp, p.err, p.failed, p.loading = bmp, err, time.Now(), false })
	}()
	return p
}

// FetchURL gets url's bytes, with the daemon's token on its own URLs.
func FetchURL(url string) ([]byte, error) { return fetchURL(url) }

func fetchURL(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if Headers != nil {
		for k, v := range Headers(url) {
			req.Header.Set(k, v)
		}
	}
	res, err := webClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, &core.ApiError{Status: res.StatusCode, Message: res.Status}
	}
	return io.ReadAll(io.LimitReader(res.Body, 32<<20))
}

// NetBitmap is the picture at url, nil while it loads or when it failed.
func NetBitmap(url string) (*ui.Bitmap, bool) {
	p := loadPicture(url, func() ([]byte, error) { return fetchURL(url) })
	return p.bmp, p.err != nil
}

// FileBitmap is the picture in the file at path.
func FileBitmap(path string) (*ui.Bitmap, bool) {
	p := loadPicture("file:"+path, func() ([]byte, error) { return os.ReadFile(path) })
	return p.bmp, p.err != nil
}

// ForgetPicture drops a cached picture, for one that changed.
func ForgetPicture(key string) {
	picturesMu.Lock()
	delete(pictures, key)
	delete(pictures, "file:"+key)
	picturesMu.Unlock()
}

// NetImage is Image.network(fit: cover) filling its parent, with fallback
// built while it loads and after it failed (errorBuilder).
func NetImage(c *ui.Context, url string, fallback func()) {
	bmp, failed := NetBitmap(url)
	if bmp == nil || failed {
		if fallback != nil {
			fallback()
		}
		return
	}
	ui.Image(c, bmp).Fit(ui.Cover).Absolute().Top(0).Left(0).Right(0).Bottom(0)
}
