package utils

import (
	"net/http"
	"encoding/json"
	"github.com/pkg/errors"
	"fmt"
	"io/ioutil"
)

func GET(url string) ([]ConnData, error) {
	rr := RResult{}
	resp, err := http.Get(url)
	if err != nil {
		return rr.Data, err
	}
	data, err := ioutil.ReadAll(resp.Body)
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
	data, err := ioutil.ReadAll(resp.Body)
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
	if uri == nil || len(uri) == 0 {
		return url
	}
	for _, s := range uri {
		url = fmt.Sprint(url, s)
	}
	return url
}
