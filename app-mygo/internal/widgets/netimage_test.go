package widgets

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A failed load is not kept: shown again after pictureRetry, the image
// loads anew, as Image.network does once its cache dropped the failure.
func TestNetBitmapRetriesAFailure(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "slow source", http.StatusBadGateway)
			return
		}
		w.Write(buf.Bytes())
	}))
	defer srv.Close()
	wait := func() {
		t.Helper()
		for i := 0; i < 200; i++ {
			picturesMu.Lock()
			loading := pictures[srv.URL].loading
			picturesMu.Unlock()
			if !loading {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("still loading")
	}
	NetBitmap(srv.URL)
	wait()
	if bmp, failed := NetBitmap(srv.URL); bmp != nil || !failed || calls.Load() != 1 {
		t.Fatalf("first load: bitmap %v, failed %v, %d calls", bmp != nil, failed, calls.Load())
	}
	picturesMu.Lock()
	pictures[srv.URL].failed = time.Now().Add(-pictureRetry)
	picturesMu.Unlock()
	NetBitmap(srv.URL)
	wait()
	if bmp, failed := NetBitmap(srv.URL); bmp == nil || failed || calls.Load() != 2 {
		t.Fatalf("retry: bitmap %v, failed %v, %d calls", bmp != nil, failed, calls.Load())
	}
}
