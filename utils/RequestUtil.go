package utils

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
)

func GET(url string) ([]ConnData, error) {
	rr := RResult{}
	resp, err := http.Get(url)
	if err != nil {
		return rr.Data, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return rr.Data, err
	}
	if data == nil {
		return rr.Data, nil
	}
	if string(data) == "" {
		return rr.Data, nil
	}
	err = json.Unmarshal(data, &rr)
	if err == nil && rr.Code != 0 {
		err = errors.New(fmt.Sprint("返回码错误：", rr.Code))
	}
	return rr.Data, err
}

func POST(url string) error {
	request, e := http.NewRequest("POST", url, nil)
	if e != nil {
		return e
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := Client.Do(request)
	//resp, err := http.Post(url, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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
