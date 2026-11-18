package core

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/walkerkiller/coursive/app-mygo/internal/l10n"
)

// ApiError is a daemon reply with an error status.
type ApiError struct {
	Status  int
	Message string
	Captcha bool
}

func (e *ApiError) Error() string { return e.Message }

// Api is the HTTP client for the daemon, and the agent host behind /agent.
type Api struct {
	BaseURL string
	Token   string
	client  *http.Client
}

// NewApi is a client for the daemon at base.
func NewApi(base, token string) *Api {
	return &Api{BaseURL: base, Token: token, client: &http.Client{}}
}

// Esc is Uri.encodeComponent.
func Esc(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }

// URI is the daemon URL of path with a query.
func (a *Api) URI(path string, query map[string]string) string {
	u := a.BaseURL + path
	if len(query) > 0 {
		v := url.Values{}
		for k, q := range query {
			v.Set(k, q)
		}
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		u += sep + v.Encode()
	}
	return u
}

// FileURL is a library file's URL for players and image loaders.
func (a *Api) FileURL(rel string) string {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = Esc(p)
	}
	return a.BaseURL + "/files/" + strings.Join(parts, "/") + "?token=" + a.Token
}

// Call is one request by method; the path may carry a query string. GET and
// DELETE send no body; the others send body, or {} without one.
func (a *Api) Call(method, path string, body any) (any, error) {
	if method == "GET" || method == "DELETE" {
		body = nil
	} else if body == nil {
		body = J{}
	}
	return a.send(method, path, body, 90*time.Second)
}

func (a *Api) send(method, path string, body any, timeout time.Duration) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var rd io.Reader
	if body != nil {
		rd = strings.NewReader(Encode(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, a.BaseURL+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return readReply(res)
}

func readReply(res *http.Response) (any, error) {
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var data any
	if len(b) > 0 {
		if data, err = Decode(b); err != nil {
			data = string(b)
		}
	}
	if res.StatusCode >= 400 {
		msg := fmt.Sprintf("HTTP %d", res.StatusCode)
		m := Obj(data)
		if m != nil && m["error"] != nil {
			msg = m.S("error")
		}
		return nil, &ApiError{Status: res.StatusCode, Message: msg, Captcha: m != nil && m["captcha"] == true}
	}
	return data, nil
}

func withQuery(path string, q map[string]string) string {
	if len(q) == 0 {
		return path
	}
	v := url.Values{}
	for k, s := range q {
		v.Set(k, s)
	}
	return path + "?" + v.Encode()
}

func (a *Api) Get(path string, q ...map[string]string) (any, error) {
	if len(q) > 0 {
		path = withQuery(path, q[0])
	}
	return a.send("GET", path, nil, 90*time.Second)
}

func (a *Api) Post(path string, body any) (any, error) {
	if body == nil {
		body = J{}
	}
	return a.send("POST", path, body, 90*time.Second)
}

// PostWait is Post with a longer timeout.
func (a *Api) PostWait(path string, body any, timeout time.Duration) (any, error) {
	if body == nil {
		body = J{}
	}
	return a.send("POST", path, body, timeout)
}

func (a *Api) Put(path string, body any) (any, error) {
	return a.send("PUT", path, body, 90*time.Second)
}

func (a *Api) Patch(path string, body any) (any, error) {
	return a.send("PATCH", path, body, 90*time.Second)
}

func (a *Api) Delete(path string) (any, error) { return a.send("DELETE", path, nil, 90*time.Second) }

// GetJ is Get whose reply is an object (an empty one when it is not).
func (a *Api) GetJ(path string) (J, error) {
	v, err := a.Get(path)
	if err != nil {
		return nil, err
	}
	if m := Obj(v); m != nil {
		return m, nil
	}
	return J{}, nil
}

func (a *Api) PreviewFolderSync(settings J) (any, error) {
	return a.Post("/api/sync/folder/preview", settings)
}

func (a *Api) ConfigureFolderSync(settings J) (any, error) {
	body := J{"kind": "folder"}
	for k, v := range settings {
		body[k] = v
	}
	return a.Put("/api/sync", body)
}

func (a *Api) SyncNow() (any, error) { return a.Post("/api/sync/run", nil) }

func (a *Api) LibraryCatalog(courseID string) (any, error) {
	if courseID == "" {
		return a.Get("/api/library/catalog")
	}
	return a.Get("/api/library/catalog?course_id=" + Esc(courseID))
}

func (a *Api) CreateLocalCourse(title, teacher string) (any, error) {
	return a.Post("/api/local-courses", J{"title": title, "teacher": teacher})
}

// ImportLocalVideo streams a video file into a local course.
func (a *Api) ImportLocalVideo(course, name string, data io.Reader, length int64) (any, error) {
	return a.Upload("/api/courses/"+Esc(course)+"/local-videos", map[string]string{"name": name}, data, length)
}

func lp(lecture string) string { return "/api/lectures/" + Esc(lecture) }

// UploadStudyAttachment posts a selection's PNG with its JSON as a form.
func (a *Api) UploadStudyAttachment(lecture string, png []byte, selection J) (any, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("selection", Encode(selection))
	f, _ := w.CreateFormFile("image", "selection.png")
	_, _ = f.Write(png)
	_ = w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+lp(lecture)+"/study-canvas/attachments", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return readReply(res)
}

func (a *Api) StudyAttachmentURL(lecture, id string) string {
	return a.URI(lp(lecture)+"/study-canvas/attachments/"+Esc(id)+"/image", map[string]string{"token": a.Token})
}

func (a *Api) SearchStudySources(lecture, query string) (any, error) {
	return a.Post(lp(lecture)+"/study-canvas/sources/search", J{"query": query})
}

func (a *Api) ReadStudySource(lecture string, source J, offset, limit int) (any, error) {
	return a.Post(lp(lecture)+"/study-canvas/sources/read", J{"source": source, "offset": offset, "limit": limit})
}

func (a *Api) ResolveStudySource(lecture string, source J) (any, error) {
	return a.Post(lp(lecture)+"/study-canvas/sources/resolve", J{"source": source})
}

func (a *Api) StudyConversation(lecture, id string) (any, error) {
	return a.Get(lp(lecture) + "/study-canvas/conversations/" + Esc(id))
}

func (a *Api) SaveStudyConversation(lecture, id string, data J) (any, error) {
	return a.Put(lp(lecture)+"/study-canvas/conversations/"+Esc(id), data)
}

func (a *Api) ImportStudyMaterial(lecture, path string) (any, error) {
	return a.Post(lp(lecture)+"/study-canvas/material", J{"path": path})
}

func (a *Api) StudyMediaURL(id, object string) string {
	return a.URI(lp(id)+"/study-canvas/media-file/"+Esc(object), map[string]string{"token": a.Token})
}

func (a *Api) PrepareStudyMedia(id string, query J) (any, error) {
	return a.Post(lp(id)+"/study-canvas/media", query)
}

func (a *Api) StudyMediaTask(id, task string) (any, error) {
	return a.Get(lp(id) + "/study-canvas/media/" + Esc(task))
}

func (a *Api) CancelStudyMedia(id, task string) (any, error) {
	return a.Delete(lp(id) + "/study-canvas/media/" + Esc(task))
}

func (a *Api) StudyPDFPage(id, source string, page int) (any, error) {
	return a.Get(fmt.Sprintf("%s/study-canvas/pdf-page?source=%s&page=%d", lp(id), Esc(source), page))
}

func (a *Api) StudyCanvas(id string) (any, error) { return a.Get(lp(id) + "/study-canvas") }

func (a *Api) StudyContext(id string, selection J) (any, error) {
	return a.Post(lp(id)+"/study-canvas/context", selection)
}

func (a *Api) StartCanvasRun(request J) (any, error) { return a.Post("/agent/canvas/runs", request) }
func (a *Api) CanvasRun(id string) (any, error)      { return a.Get("/agent/canvas/runs/" + Esc(id)) }
func (a *Api) CancelCanvasRun(id string) (any, error) {
	return a.Delete("/agent/canvas/runs/" + Esc(id))
}

// SaveStudyCanvas saves the whole document.
func (a *Api) SaveStudyCanvas(id string, document J) (any, error) {
	return a.Put(lp(id)+"/study-canvas", document)
}

func (a *Api) ImportStudyAsset(id, name string, data io.Reader, length int64) (any, error) {
	return a.Upload(lp(id)+"/study-canvas/assets", map[string]string{"name": name}, data, length)
}

func (a *Api) ImportStudyCourseware(id, path string) (any, error) {
	return a.Post(lp(id)+"/study-canvas/assets?path="+Esc(path), nil)
}

// PreviewExportPDF renders a page of PDF bytes the app built.
func (a *Api) PreviewExportPDF(pdf []byte, page int) (any, error) {
	return a.Upload("/api/export/pdf-preview", map[string]string{"page": fmt.Sprint(page)}, bytes.NewReader(pdf), int64(len(pdf)))
}

func (a *Api) TranscriptReview(id string) (any, error) {
	return a.Get("/agent/transcript-review/" + Esc(id))
}

func (a *Api) StartTranscriptReview(id string) (any, error) {
	return a.Post("/agent/transcript-review/"+Esc(id), nil)
}

func (a *Api) CancelTranscriptReview(id string) (any, error) {
	return a.Delete("/agent/transcript-review/" + Esc(id))
}

func (a *Api) DismissTranscriptSuggestion(id, suggestion string) (any, error) {
	return a.Patch(lp(id)+"/transcript-review", J{"suggestion_id": suggestion})
}

// StudyImageBytes reads an approved material file, at most 8 MiB.
func (a *Api) StudyImageBytes(rel string) ([]byte, error) {
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = Esc(p)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", a.BaseURL+"/files/"+strings.Join(parts, "/"), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, &ApiError{Status: res.StatusCode, Message: l10n.StudyVisualReadFailed}
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 8<<20+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 8<<20 {
		return nil, errors.New(l10n.StudyVisualFileTooLarge)
	}
	return b, nil
}

// Upload streams data as the raw request body.
func (a *Api) Upload(path string, query map[string]string, data io.Reader, length int64) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", a.URI(path, query), data)
	if err != nil {
		return nil, err
	}
	req.ContentLength = length
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return readReply(res)
}

// Download streams a reply into dest through a temporary file, reporting
// the bytes written so far.
func (a *Api) Download(path string, query map[string]string, dest string, progress func(n int64)) (int64, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", a.URI(path, query), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	res, err := a.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		_, err := readReply(res)
		return 0, err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	var n int64
	buf := make([]byte, 256<<10)
	for {
		k, rerr := res.Body.Read(buf)
		if k > 0 {
			if _, werr := f.Write(buf[:k]); werr != nil {
				f.Close()
				os.Remove(tmp)
				return n, werr
			}
			n += int64(k)
			if progress != nil {
				progress(n)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(tmp)
			return n, rerr
		}
	}
	if err := f.Close(); err != nil {
		return n, err
	}
	_ = os.Remove(dest)
	return n, os.Rename(tmp, dest)
}

// SSE streams Server-Sent Events as decoded objects to fn until the
// connection ends, the context is cancelled or an error occurs.
func (a *Api) SSE(ctx context.Context, method, path string, body any, fn func(J)) error {
	var rd io.Reader
	if body != nil {
		rd = strings.NewReader(Encode(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, a.BaseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	res, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		_, err := readReply(res)
		return err
	}
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 64<<20)
	var data []string
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if len(data) > 0 {
				if v, err := Decode([]byte(strings.Join(data, "\n"))); err == nil {
					if m := Obj(v); m != nil {
						fn(m)
					}
				}
				data = data[:0]
			}
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			data = append(data, line[6:])
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
