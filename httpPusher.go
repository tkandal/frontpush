package frontpush

import (
	"bytes"
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"time"
)

/*
 * Copyright (c) 2019 Norwegian University of Science and Technology
 */

// HTTPPusher uses HTTP to push to endpoint
type HTTPPusher struct {
	URL          string
	User         string
	Pass         string
	Method       string
	Logger       *zap.SugaredLogger
	Headers      map[string][]string
	Timeout      time.Duration
	client       *http.Client
	HistogramVec *prometheus.HistogramVec
}

// Push pushes cards to destination endpoint
func (hp *HTTPPusher) Push(r io.Reader) (io.Reader, error) {
	u, err := url.Parse(hp.URL)
	if err != nil {
		hp.Logger.Errorw(fmt.Sprintf("URL %s is not parseable", hp.URL), "error", err)
		return nil, err
	}

	status := http.StatusOK
	defer func(s time.Time) {
		if hp.HistogramVec != nil && u != nil {
			hp.HistogramVec.With(prometheus.Labels{
				"route":  u.Path,
				"method": hp.Method,
				"status": fmt.Sprintf("%d", status),
			}).Observe(float64(time.Now().Sub(s).Nanoseconds() / int64(time.Millisecond)))
		}
	}(time.Now())

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(hp.Timeout))
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, hp.Method, hp.URL, r)
	if err != nil {
		status = http.StatusInternalServerError
		hp.Logger.Errorw(fmt.Sprintf("% %s new request failed", hp.Method, hp.URL), "error", err)
		return nil, err
	}

	if len(hp.User) > 0 || len(hp.Pass) > 0 {
		req.SetBasicAuth(hp.User, hp.Pass)
	}

	if len(hp.Headers) > 0 {
		for key, values := range hp.Headers {
			for i, value := range values {
				if i > 0 {
					req.Header.Add(key, value)
				} else {
					req.Header.Set(key, value)
				}
			}
		}
	}

	if hp.client == nil {
		hp.client = &http.Client{}
	}

	resp, err := hp.client.Do(req)
	if err != nil {
		status = http.StatusInternalServerError
		hp.Logger.Errorw(fmt.Sprintf("%s %s request failed", hp.Method, hp.URL), "error", err)
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	respBuf := &bytes.Buffer{}
	if _, err := respBuf.ReadFrom(resp.Body); err != nil {
		status = http.StatusInternalServerError
		hp.Logger.Errorw(fmt.Sprintf("%s %s read response failed", hp.Method, hp.URL), "error", err)
		return nil, err
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		status = resp.StatusCode
		hp.Logger.Errorf("%s %s returned status-code %d", hp.Method, hp.URL, resp.StatusCode)
		hp.Logger.Error(respBuf.String())
		return nil, fmt.Errorf("%s %s returned status-code %d", hp.Method, hp.URL, resp.StatusCode)
	}

	return respBuf, nil
}
