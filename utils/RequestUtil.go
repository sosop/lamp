package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

func GET(url string) ([]ConnData, error) {
	rr := RResult{}
	resp, err := http.Get(url)
	if err != nil {
		return rr.Data, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return rr.Data, err
	}
	err = json.Unmarshal(data, &rr)
	if err == nil && rr.Code != 0 {
		err = errors.New(fmt.Sprint("返回码错误：", rr.Code))
	}
	return rr.Data, err
}

func POST(url string) error {

	resp, err := http.Post(url, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	pr := PResult{}
	err = json.Unmarshal(data, &pr)
	if err == nil && pr.Code != 0 {
		err = errors.New(fmt.Sprint("返回码错误：", pr.Code))
	}
	return err
}

func MakeUrl(domain string, uri ...string) string {
	url := fmt.Sprint("http://", domain)
	if len(uri) == 0 {
		return url
	}
	for _, s := range uri {
		url = fmt.Sprint(url, s)
	}
	return url
}
