package push

import (
	"bytes"
	"context"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"time"
)

/*
 * Copyright (c) 2019 Norwegian University of Science and Technology
 */

// HTTPPusher uses HTTP to push to endpoint
type HTTPPusher struct {
	URL     string
	User    string
	Pass    string
	Method  string
	Logger  *zap.SugaredLogger
	Headers map[string]string
	Timeout time.Duration
	client  *http.Client
}

// Push pushes cards to destination endpoint
func (hp *HTTPPusher) Push(r io.Reader) (io.Reader, error) {
	req, err := http.NewRequest(hp.Method, hp.URL, r)
	if err != nil {
		hp.Logger.Errorw("new request failed", "error", err.Error())
		return nil, err
	}

	if len(hp.User) > 0 && len(hp.Pass) > 0 {
		req.SetBasicAuth(hp.User, hp.Pass)
	}

	if len(hp.Headers) >  0 {
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
		hp.Logger.Errorw("request failed", "error", err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	respBuf := &bytes.Buffer{}
	_, err = respBuf.ReadFrom(resp.Body)
	if err != nil {
		hp.Logger.Errorw("read response failed", "error", err.Error())
		return nil, err
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		hp.Logger.Errorf("%s %s returned status-code %d", hp.Method, hp.URL, resp.StatusCode)
		hp.Logger.Error(respBuf.String())
		return nil, fmt.Errorf("%s %s returned status-code %d", hp.Method, hp.URL, resp.StatusCode)
	}

	return respBuf, nil
}
