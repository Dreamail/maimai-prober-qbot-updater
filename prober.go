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

type Status struct {
	Diff int
	Err  error
}

func (b *Bot) UpdateScore(idx, user, passwd string, async bool) (chan Status, []Status) {
	status := make(chan Status, 5)
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		i := i
		go func() {
			err := b.updateScore(idx, user, passwd, i)
			if err != nil {
				status <- Status{
					Diff: i,
					Err:  err,
				}
				return
			}
			status <- Status{
				Diff: i,
				Err:  nil,
			}
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

	errStats := []Status{}
	for i := 0; i < 5; i++ {
		stat := <-status
		if stat.Err != nil {
			errStats = append(errStats, stat)
		}
	}
	close(status)
	if len(errStats) != 0 {
		return nil, errStats
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
