package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"
)

// Quote is one line from hitokoto.cn (一言).
type Quote struct{ Text, From, Who string }

// Source is "—— 作者《出处》".
func (q Quote) Source() string {
	f := ""
	if q.From != "" {
		if strings.HasPrefix(q.From, "《") {
			f = q.From
		} else {
			f = "《" + q.From + "》"
		}
	}
	w := strings.TrimSpace(q.Who)
	if w == "" && f == "" {
		return ""
	}
	return "—— " + w + f
}

func (q Quote) json() J { return J{"text": q.Text, "from": q.From, "who": q.Who} }

func quoteOf(m J) Quote { return Quote{m.S("text"), m.S("from"), m.S("who")} }

var fallbackQuotes = []Quote{
	{"学而不思则罔，思而不学则殆。", "论语", ""},
	{"博学之，审问之，慎思之，明辨之，笃行之。", "礼记·中庸", ""},
	{"路漫漫其修远兮，吾将上下而求索。", "离骚", "屈原"},
	{"纸上得来终觉浅，绝知此事要躬行。", "冬夜读书示子聿", "陆游"},
	{"不积跬步，无以至千里；不积小流，无以成江海。", "荀子·劝学", ""},
}

var currentQuote *Quote

// CurrentQuote is the quote fetched this session, if any.
func CurrentQuote() *Quote { return currentQuote }

// CachedQuote is the quote of a previous launch, for an instant first frame.
func CachedQuote() *Quote {
	raw, ok := Prefs.String("hitokoto.last")
	if !ok {
		return nil
	}
	v, err := Decode([]byte(raw))
	if err != nil || Obj(v) == nil {
		return nil
	}
	q := quoteOf(Obj(v))
	return &q
}

// FetchQuote is Hitokoto.get: once per session unless refresh. It blocks;
// call it off the UI thread and set the result with SetQuote.
func FetchQuote(types []string, refresh bool) Quote {
	if currentQuote != nil && !refresh {
		return *currentQuote
	}
	var params []string
	for _, t := range types {
		params = append(params, "c="+t)
	}
	params = append(params, "encode=json", "max_length=48")
	q, err := fetchHitokoto("https://v1.hitokoto.cn/?" + strings.Join(params, "&"))
	if err == nil {
		Prefs.SetString("hitokoto.last", Encode(q.json()))
		return q
	}
	if refresh || currentQuote == nil {
		if !refresh {
			if c := CachedQuote(); c != nil {
				return *c
			}
		}
		return fallbackQuotes[rand.IntN(len(fallbackQuotes))]
	}
	return *currentQuote
}

// SetQuote records the session's quote, on the UI thread.
func SetQuote(q Quote) { currentQuote = &q }

func fetchHitokoto(url string) (Quote, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Quote{}, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return Quote{}, err
	}
	v, err := Decode(b)
	m := Obj(v)
	if err != nil || m == nil {
		return Quote{}, errors.New("bad reply")
	}
	q := Quote{strings.TrimSpace(m.S("hitokoto")), strings.TrimSpace(m.S("from")), strings.TrimSpace(m.S("from_who"))}
	if q.Text == "" {
		return Quote{}, errors.New("empty")
	}
	return q, nil
}
