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
	Headers      map[string]string
	Timeout      time.Duration
	client       *http.Client
	HistogramVec *prometheus.HistogramVec
}

// Push pushes cards to destination endpoint
func (hp *HTTPPusher) Push(r io.Reader) (io.Reader, error) {
	u, err := url.Parse(hp.URL)
	if err != nil {
		hp.Logger.Warnw("URL not parseable", "error", err.Error())
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

	req, err := http.NewRequest(hp.Method, hp.URL, r)
	if err != nil {
		status = http.StatusInternalServerError
		hp.Logger.Errorw("new request failed", "error", err.Error())
		return nil, err
	}

	if len(hp.User) > 0 && len(hp.Pass) > 0 {
		req.SetBasicAuth(hp.User, hp.Pass)
	}

	if len(hp.Headers) > 0 {
		for k, v := range hp.Headers {
			req.Header.Set(k, v)
		}
	}

	if hp.client == nil {
		hp.client = &http.Client{}
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(hp.Timeout))
	defer cancel()

	resp, err := hp.client.Do(req.WithContext(ctx))
	if err != nil {
		status = http.StatusInternalServerError
		hp.Logger.Errorw("request failed", "error", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	respBuf := &bytes.Buffer{}
	_, err = respBuf.ReadFrom(resp.Body)
	if err != nil {
		status = http.StatusInternalServerError
		hp.Logger.Errorw("read response failed", "error", err.Error())
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
