package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"sync"
)

func (b *Bot) UpdateScore(idx, user, passwd string, async bool) (chan string, error) {
	status := make(chan string, 5)
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		i := i
		go func() {
			err := b.updateScore(idx, user, passwd, i)
			if err != nil {
				status <- fmt.Sprintf("diff: %d err: %s", i, err.Error())
				return
			}
			status <- fmt.Sprintf("diff: %d success", i)
			wg.Done()
		}()
	}
	if async {
		go func() {
			wg.Wait()
			close(status)
		}()
		return status, nil
	}

	errStr := ""
	for i := 0; i < 5; i++ {
		stat := <-status
		if strings.Contains(stat, "err: ") {
			errStr = errStr + stat + "\n"
		}
	}
	close(status)
	if errStr != "" {
		return nil, errors.New(errStr)
	}
	return nil, nil
}
func (b *Bot) updateScore(idx, user, passwd string, diff int) error {
	result := []string{"", ""}
	var err error
	errCh := make(chan error, 2)
	defer close(errCh)

	htmlReg := regexp.MustCompile(`<html.*>([\s\S]*)<\/html>`)
	spaceReg := regexp.MustCompile(`\s+`)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			result[i], err = b.GetFriendVS(idx, fmt.Sprint(i), diff, true)
			errCh <- err
			if err != nil {
				return
			}
			result[i] = spaceReg.ReplaceAllString(htmlReg.FindStringSubmatch(result[i])[1], " ")
		}()
	}

	errStr := ""
	for i := 0; i < 2; i++ {
		err = <-errCh
		if err != nil {
			errStr = errStr + ", " + err.Error()
		}
	}
	if errStr != "" {
		return errors.New(errStr)
	}

	records, err := ParseRecords(result[0], result[1], diff)
	if err != nil && err.Error() != "record was not found" {
		return err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Jar: jar,
	}
	loginResp, err := client.Post(
		"https://www.diving-fish.com/api/maimaidxprober/login",
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"username": "%s", "password": "%s"}`, user, passwd)),
	)
	if err != nil {
		return err
	}
	if loginResp.StatusCode != 200 {
		return errors.New("login failed: " + loginResp.Status)
	}

	body, err := json.Marshal(records)
	client.Post(
		"https://www.diving-fish.com/api/maimaidxprober/player/update_records",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}

	return nil
}
