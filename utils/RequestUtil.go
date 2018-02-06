package utils

import (
	"net/http"
	"net/http/httputil"
	"encoding/json"
	"github.com/pkg/errors"
	"fmt"
)

func GET(url string) ([]ConnData, error) {
	rr := RResult{}
	resp, err := http.Get(url)
	if err != nil {
		return rr.Data, err
	}
	data, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return rr.Data, err
	}
	err = json.Unmarshal(data, &rr)
	if rr.Code != 0 {
		err = errors.New(fmt.Sprint("返回码错误：", rr.Code))
	}
	return rr.Data, err
}

func POST(url string) ([]ConnData, error) {
	rr := RResult{}
	resp, err := http.Post(url, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return rr.Data, err
	}
	data, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return rr.Data, err
	}
	err = json.Unmarshal(data, &rr)
	if rr.Code != 0 {
		err = errors.New(fmt.Sprint("返回码错误：", rr.Code))
	}
	return rr.Data, err
}

func MakeUrl(domain string, uri ...string) string {
	url := fmt.Sprint("http://", domain)
	if uri == nil || len(uri) == 0 {
		return url
	}
	for _, s := range uri {
		url = fmt.Sprint(url, s)
	}
	return url
}
